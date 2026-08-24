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

// 本文件提供 gorm 标签到 opao db 标签的迁移映射层：
// 从既有 gorm 模型平滑迁移时，按优先级解析列名与主键/自增属性，
// 并对无法迁移的选项以 fail-open 方式汇总为警告，绝不中断流程。

package support

import (
	"fmt"
	"strings"
	"unicode"
)

// SnakeCase 驼峰转蛇形：UserName→user_name；连续大写 HTTPCode→http_code；
// APIKey→api_key；ID→id；UserID→user_id。
// 规则：大写字母前需要补下划线，当且仅当前一字符为小写或数字，
// 或前一字符为大写且后一字符为小写（连续大写的末位边界）。
func SnakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(runes) + 4)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			prevIsLowerOrDigit := unicode.IsLower(prev) || unicode.IsDigit(prev)
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prev != '_' && (prevIsLowerOrDigit || (unicode.IsUpper(prev) && nextIsLower)) {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// GormTagInfo 解析后的 gorm 标签信息。
type GormTagInfo struct {
	Column        string   // column:xxx 指定列名，可为空
	PrimaryKey    bool     // primaryKey
	AutoIncrement bool     // autoIncrement
	Ignore        bool     // "-" 表示忽略字段
	Unknown       []string // 不支持的选项名（size/index/type/default 等）
}

// gormKnownOptions 本映射层可识别并处理的 gorm 选项名集合。
var gormKnownOptions = map[string]bool{
	"column":        true,
	"primaryKey":    true,
	"autoIncrement": true,
	"-":             true,
}

// splitGormOption 将单个 gorm 选项拆分为键与值。
// 同时兼容 gorm 官方的冒号分隔（column:name）与等号分隔（column=name）两种写法，
// 以先出现的分隔符为准。
func splitGormOption(opt string) (key, value string) {
	if i := strings.IndexAny(opt, ":="); i >= 0 {
		return strings.TrimSpace(opt[:i]), strings.TrimSpace(opt[i+1:])
	}
	return strings.TrimSpace(opt), ""
}

// ParseGormTag 解析 `gorm:"..."` 标签内容（不含 key 本身，传入引号内文本）。
// 分号分隔多个选项；形如 key 或 key=value（亦兼容 gorm 官方的 key:value 写法）。
// fail-open：未知选项进 Unknown 不报错，由调用方决定如何提示。
func ParseGormTag(body string) GormTagInfo {
	var info GormTagInfo
	for _, raw := range strings.Split(body, ";") {
		opt := strings.TrimSpace(raw)
		if opt == "" {
			continue
		}
		key, value := splitGormOption(opt)
		switch {
		case key == "-":
			info.Ignore = true
		case key == "column":
			if value == "" {
				// column 缺失取值无法生效，按未知选项记录，fail-open 不报错
				info.Unknown = append(info.Unknown, key)
			} else {
				info.Column = value
			}
		case key == "primaryKey":
			info.PrimaryKey = true
		case key == "autoIncrement":
			info.AutoIncrement = true
		case !gormKnownOptions[key]:
			info.Unknown = append(info.Unknown, key)
		}
	}
	return info
}

// ColumnMapping 决策结果。
type ColumnMapping struct {
	Column        string
	PrimaryKey    bool
	AutoIncrement bool
	Skip          bool     // 该字段不参与映射
	Warnings      []string // 识别到但不支持的特性提示
}

// ResolveColumn 按优先级解析列名与属性：
//  1. opao db 标签值非空且非 "-" → 直接使用（autoIncrement 由 option 标签另行处理，此处不管）
//  2. db 标签为 "-": Skip=true
//  3. 无 db 标签且有 gorm column → 用之；primaryKey/autoIncrement 继承
//  4. 都没有：useSnake ? SnakeCase(fieldName) : ""（由调用方决定跳过或报错）
//
// 另：gorm 的 "-" → Skip 并给 Warning 提示改用 opao 的 db:"-"；
// 当 gorm 含未支持选项（Unknown 非空）时汇总一条含「未迁移」的 Warning，
// 不对 default/current_timestamp 等做语义判断。
func ResolveColumn(dbTag string, hasDB bool, gorm GormTagInfo, fieldName string, useSnake bool) ColumnMapping {
	var m ColumnMapping
	// 先汇总不可迁移的 gorm 选项，无论后续走哪个分支都应提示
	if len(gorm.Unknown) > 0 {
		m.Warnings = append(m.Warnings,
			fmt.Sprintf("存在未迁移的 gorm 选项: %s", strings.Join(gorm.Unknown, ", ")))
	}
	// 优先级 1/2：opao db 标签拥有最终决定权
	if hasDB {
		switch dbTag {
		case "-":
			m.Skip = true
			return m
		case "":
			// 空值视为 opao 未表态，继续向下回退到 gorm 标签
		default:
			m.Column = dbTag
			return m
		}
	}
	// gorm 的 "-" 忽略字段：Skip 并提示改用 opao 的写法
	if gorm.Ignore {
		m.Skip = true
		m.Warnings = append(m.Warnings, `检测到 gorm:"-" 忽略字段，opao 请改用 db:"-"`)
		return m
	}
	// 优先级 3：无 db 标签但有 gorm column → 直接使用并继承主键/自增属性
	if gorm.Column != "" {
		m.Column = gorm.Column
		m.PrimaryKey = gorm.PrimaryKey
		m.AutoIncrement = gorm.AutoIncrement
		return m
	}
	// 优先级 4：两者皆无，交由调用方决定跳过或报错；仍继承主键/自增属性
	if useSnake {
		m.Column = SnakeCase(fieldName)
	}
	m.PrimaryKey = gorm.PrimaryKey
	m.AutoIncrement = gorm.AutoIncrement
	return m
}
