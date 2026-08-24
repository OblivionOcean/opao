// Copyright 2024 OblivionOcean
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package support

import (
	"reflect"
	"strings"
	"testing"
)

func TestSnake(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ID", "id"},
		{"UserID", "user_id"},
		{"HTTPCode", "http_code"},
		{"APIKey", "api_key"},
		{"URL", "url"},
		{"Name", "name"},
		{"UserName", "user_name"},
		{"userName", "user_name"},
		{"Already_Snake", "already_snake"},
		{"", ""},
	}
	for _, c := range cases {
		if got := SnakeCase(c.in); got != c.want {
			t.Errorf("SnakeCase(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

func TestGormTag(t *testing.T) {
	cases := []struct {
		name string
		body string
		want GormTagInfo
	}{
		{
			name: "column加未知项",
			body: "column:name;size:128;index",
			want: GormTagInfo{Column: "name", Unknown: []string{"size", "index"}},
		},
		{
			name: "主键自增",
			body: "primaryKey;autoIncrement",
			want: GormTagInfo{PrimaryKey: true, AutoIncrement: true},
		},
		{
			name: "忽略字段",
			body: "-",
			want: GormTagInfo{Ignore: true},
		},
		{
			name: "空串",
			body: "",
			want: GormTagInfo{},
		},
		{
			name: "含多个未知项",
			body: "column:id;type:varchar(10);default:0",
			want: GormTagInfo{Column: "id", Unknown: []string{"type", "default"}},
		},
		{
			name: "等号写法与空白容忍",
			body: " column = email ; index ",
			want: GormTagInfo{Column: "email", Unknown: []string{"index"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ParseGormTag(c.body)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ParseGormTag(%q) = %+v, 期望 %+v", c.body, got, c.want)
			}
		})
	}
}

func TestResolveColumn(t *testing.T) {
	cases := []struct {
		name      string
		dbTag     string
		hasDB     bool
		gorm      GormTagInfo
		fieldName string
		useSnake  bool
		want      ColumnMapping
	}{
		{
			name:      "优先级1_opao_db标签直接生效",
			dbTag:     "user_name",
			hasDB:     true,
			gorm:      GormTagInfo{Column: "ignored", PrimaryKey: true, Unknown: []string{"size"}},
			fieldName: "UserName",
			useSnake:  true,
			want: ColumnMapping{
				Column:   "user_name",
				Warnings: []string{"size"},
			},
		},
		{
			name:  "优先级2_db为减号跳过",
			dbTag: "-",
			hasDB: true,
			want:  ColumnMapping{Skip: true},
		},
		{
			name:      "db空值回退gorm_column并继承属性",
			dbTag:     "",
			hasDB:     true,
			gorm:      GormTagInfo{Column: "created_at", PrimaryKey: true, AutoIncrement: true},
			fieldName: "CreatedAt",
			useSnake:  false,
			want:      ColumnMapping{Column: "created_at", PrimaryKey: true, AutoIncrement: true},
		},
		{
			name:      "优先级3_无db有gorm_column继承主键自增",
			hasDB:     false,
			gorm:      GormTagInfo{Column: "order_id", PrimaryKey: true, AutoIncrement: true},
			fieldName: "OrderID",
			useSnake:  true,
			want:      ColumnMapping{Column: "order_id", PrimaryKey: true, AutoIncrement: true},
		},
		{
			name:      "优先级4_useSnake生成蛇形列名",
			hasDB:     false,
			gorm:      GormTagInfo{},
			fieldName: "UserName",
			useSnake:  true,
			want:      ColumnMapping{Column: "user_name"},
		},
		{
			name:      "优先级4_useSnake为假留空由调用方决定",
			hasDB:     false,
			fieldName: "UserName",
			useSnake:  false,
			want:      ColumnMapping{},
		},
		{
			name:      "gorm减号跳过并提示改用opao写法",
			hasDB:     false,
			gorm:      GormTagInfo{Ignore: true},
			fieldName: "DeletedAt",
			useSnake:  true,
			want:      ColumnMapping{Skip: true, Warnings: []string{`db:"-"`}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveColumn(c.dbTag, c.hasDB, c.gorm, c.fieldName, c.useSnake)
			if got.Column != c.want.Column || got.PrimaryKey != c.want.PrimaryKey ||
				got.AutoIncrement != c.want.AutoIncrement || got.Skip != c.want.Skip ||
				len(got.Warnings) != len(c.want.Warnings) {
				t.Fatalf("ResolveColumn(db=%q hasDB=%v gorm=%+v field=%q snake=%v) = %+v, 期望 %+v",
					c.dbTag, c.hasDB, c.gorm, c.fieldName, c.useSnake, got, c.want)
			}
			for _, wantSub := range c.want.Warnings {
				if !containsWarning(got.Warnings, wantSub) {
					t.Errorf("Warnings %v 中未找到期望内容 %q", got.Warnings, wantSub)
				}
			}
		})
	}

	// Warnings 文案必须包含「未迁移」字样
	warn := ResolveColumn("", false, GormTagInfo{Unknown: []string{"size", "index"}}, "X", false).Warnings
	if len(warn) == 0 || !strings.Contains(warn[0], "未迁移") {
		t.Errorf("Unknown 非空时应产生包含「未迁移」的 Warning，实际得到 %v", warn)
	}
	if !strings.Contains(warn[0], "size") || !strings.Contains(warn[0], "index") {
		t.Errorf("Warning 应汇总未迁移选项名，实际得到 %q", warn[0])
	}
}

// containsWarning 判断警告列表中是否存在包含 sub 的条目。
func containsWarning(warnings []string, sub string) bool {
	for _, w := range warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
