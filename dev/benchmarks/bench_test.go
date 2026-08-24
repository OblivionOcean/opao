package benchmarks

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite" // 注册名为 "sqlite" 的纯 Go database/sql 驱动（opao 侧使用）

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
)

// 统一模型：两侧映射同一张 users 表、同样的列。
// opao 侧用 db 标签，gorm 侧用独立结构体 + gorm 标签，保证各自惯用写法。
type BenchUser struct {
	ID   int64  `db:"id" option:"autoIncrement"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

type GUser struct {
	ID   int64  `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"column:name"`
	Age  int    `gorm:"column:age"`
}

func (GUser) TableName() string { return "users" }

const (
	benchTableName = "users"
	seedRows       = 100
	findAgeCond    = 50 // FindAll 条件 age > findAgeCond
)

const createTableSQL = `CREATE TABLE ` + benchTableName + ` (
	id   INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT    NOT NULL DEFAULT '',
	age  INTEGER NOT NULL DEFAULT 0
)`

// ---------------------------------------------------------------------------
// 组 A：纯 SQL 构建（不触库）
//
// opao 侧直接调用 support.BuildUpdate 的构建路径；
// gorm 侧用 DryRun 会话只生成 SQL 不执行。二者都衡量“把对象/条件变成 SQL”的成本。
// ---------------------------------------------------------------------------

// snapshotElems 从 Register 后的 ORM 快照取得表名、字段元数据与对象类型，
// 并按 ORM.Load 相同方式回填字段指针，使 Elem.Get() 可读取对象当前值。
func snapshotElems(b *testing.B, o *support.ORM, obj *BenchUser) (string, []support.Elem, reflect.Type) {
	table, elems, objType, err := o.Snapshot(obj)
	if err != nil {
		b.Fatalf("Snapshot: %v", err)
	}
	base := unsafe.Pointer(obj)
	for i := range elems {
		elems[i].Ptr = unsafe.Add(base, elems[i].Offset)
	}
	return table, elems, objType
}

func BenchmarkOpao_BuildUpdate(b *testing.B) {
	o := &support.ORM{}
	o.Init(nil, mysql.New) // 仅借用 MySQL 方言做 SQL 构建，conn 为 nil 不触库
	if err := o.Register(benchTableName, &BenchUser{}); err != nil {
		b.Fatalf("Register: %v", err)
	}
	u := &BenchUser{Name: "alice", Age: 30}
	table, elems, objType := snapshotElems(b, o, u)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query, args, err := support.BuildUpdate(
			mysql.Dialect, table, elems, false, u, objType,
			"id = ?", 1,
		)
		if err != nil || query == "" || len(args) == 0 {
			b.Fatalf("BuildUpdate: query=%q args=%v err=%v", query, args, err)
		}
	}
}

func BenchmarkGorm_DryRunUpdate(b *testing.B) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("gorm.Open: %v", err)
	}
	dry := gdb.Session(&gorm.Session{DryRun: true})
	updates := map[string]any{"name": "alice", "age": 30}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stmt := dry.Model(&GUser{}).Where("id = ?", 1).Updates(updates)
		if stmt.Error != nil {
			b.Fatalf("DryRun Updates: %v", stmt.Error)
		}
	}
}

// ---------------------------------------------------------------------------
// 组 B：真实往返（sqlite 内存库，单连接）
// ---------------------------------------------------------------------------

func newOpaoBenchDB(b *testing.B) *opao.Database {
	db, err := opao.NewDatabase("sqlite", "file:bench_opao?mode=memory&cache=shared")
	if err != nil {
		b.Fatalf("opao.NewDatabase: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })
	db.Conn.SetMaxOpenConns(1) // 内存库多连接不共享数据，限制为单连接
	if _, err := db.Conn.Exec(createTableSQL); err != nil {
		b.Fatalf("create table: %v", err)
	}
	if err := db.Register(benchTableName, &BenchUser{}); err != nil {
		b.Fatalf("Register: %v", err)
	}
	for i := 0; i < seedRows; i++ {
		u := &BenchUser{Name: fmt.Sprintf("user-%d", i), Age: i}
		if err := db.Load(u).Create(); err != nil {
			b.Fatalf("seed Create #%d: %v", i, err)
		}
	}
	return db
}

func newGormBenchDB(b *testing.B) *gorm.DB {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		b.Fatalf("gorm.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // 与 opao 侧保持一致的单连接
	if err := gdb.Exec(createTableSQL).Error; err != nil {
		b.Fatalf("create table: %v", err)
	}
	for i := 0; i < seedRows; i++ {
		row := &GUser{Name: fmt.Sprintf("user-%d", i), Age: i}
		if err := gdb.Create(row).Error; err != nil {
			b.Fatalf("seed Create #%d: %v", i, err)
		}
	}
	return gdb
}

func BenchmarkOpao_Roundtrip_FindAll(b *testing.B) {
	db := newOpaoBenchDB(b)
	q := db.Load(&BenchUser{})

	want, err := q.FindAll("age > ?", findAgeCond)
	if err != nil {
		b.Fatalf("warmup FindAll: %v", err)
	}
	expected := len(want)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := q.FindAll("age > ?", findAgeCond)
		if err != nil {
			b.Fatalf("FindAll: %v", err)
		}
		if len(rows) != expected {
			b.Fatalf("FindAll 返回 %d 行, 期望 %d 行", len(rows), expected)
		}
	}
}

func BenchmarkGorm_Roundtrip_FindAll(b *testing.B) {
	gdb := newGormBenchDB(b)

	var want []GUser
	if err := gdb.Where("age > ?", findAgeCond).Find(&want).Error; err != nil {
		b.Fatalf("warmup Find: %v", err)
	}
	expected := len(want)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var rows []GUser
		if err := gdb.Where("age > ?", findAgeCond).Find(&rows).Error; err != nil {
			b.Fatalf("Find: %v", err)
		}
		if len(rows) != expected {
			b.Fatalf("Find 返回 %d 行, 期望 %d 行", len(rows), expected)
		}
	}
}

func BenchmarkOpao_Roundtrip_UpdateByID(b *testing.B) {
	db := newOpaoBenchDB(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := int64(i%seedRows + 1)
		name := fmt.Sprintf("updated-%d", i)
		up := db.Load(&BenchUser{ID: id, Name: name})
		if err := up.Update("id = ?", id); err != nil {
			b.Fatalf("Update id=%d: %v", id, err)
		}
	}
}

func BenchmarkGorm_Roundtrip_UpdateByID(b *testing.B) {
	gdb := newGormBenchDB(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := i%seedRows + 1
		name := fmt.Sprintf("updated-%d", i)
		if err := gdb.Model(&GUser{}).Where("id = ?", id).Update("name", name).Error; err != nil {
			b.Fatalf("Update id=%d: %v", id, err)
		}
	}
}
