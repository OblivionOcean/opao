// 各语句类型全量对比：
// A 组【纯构建】fakeExecer 隔离 I/O —— 只测 SQL 构造与框架开销（Exec 型语句）
// B 组【真实往返】sqlite 内存库 —— 含驱动交互的端到端延迟（全部语句）
// 公平性：同模型、同参数、同机同序执行；gorm 侧统一 DryRun(构建组)/真实会话(往返组)
package benchmarks

import (
	"context"
	"database/sql"
	"github.com/OblivionOcean/opao/dev/benchmarks/model"
	"github.com/OblivionOcean/opao/dev/benchmarks/query"
	"testing"

	_ "github.com/mattn/go-sqlite3" // 裸基准用：注册名 "sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
)

type stmtUser struct {
	ID   int64  `db:"id" option:"autoIncrement"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func newOpaoStmt(t testing.TB) *mysql.MySQL {
	obj := &stmtUser{ID: 7, Name: "n", Age: 20}
	orm := &support.ORM{}
	orm.Init(nil, mysql.New)
	if err := orm.Register("su", &stmtUser{}); err != nil {
		t.Fatal(err)
	}
	o := orm.Load(obj).(*mysql.MySQL)
	o.BindExec(fakeExecer{}, context.Background())
	return o
}

func newGormDry(t testing.TB) *gorm.DB {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return gdb.Session(&gorm.Session{DryRun: true})
}

func newRealPair(t testing.TB) (*opao.Database, *gorm.DB) {
	db, err := opao.NewDatabase("sqlite3", "file:bench_stmts?mode=memory&cache=shared") // 与 gorm 同引擎(mattn)
	if err != nil {
		t.Fatal(err)
	}
	db.Conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Conn.Exec(`CREATE TABLE su (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT DEFAULT '', age INTEGER DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Register("su", &stmtUser{}); err != nil {
		t.Fatal(err)
	}
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&model.GStmtUser{}); err != nil {
		t.Fatal(err)
	}
	return db, gdb
}

// ---------- A 组：纯构建 ----------

func BenchmarkA_Create_Opao(b *testing.B) {
	o := newOpaoStmt(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = o.Create()
	}
}

func BenchmarkA_Create_Gorm(b *testing.B) {
	g := newGormDry(b)
	u := &model.GStmtUser{Name: "n", Age: 20}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Create(u)
	}
}

func BenchmarkA_Update_Opao(b *testing.B) {
	o := newOpaoStmt(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = o.Update("id = ?", 7)
	}
}

func BenchmarkA_Update_Gorm(b *testing.B) {
	g := newGormDry(b)
	u := &model.GStmtUser{ID: 7}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Model(u).Where("id = ?", 7).Updates(map[string]any{"name": "x"})
	}
}

func BenchmarkA_Delete_Opao(b *testing.B) {
	o := newOpaoStmt(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = o.Delete("id = ?", 7)
	}
}

func BenchmarkA_Delete_Gorm(b *testing.B) {
	g := newGormDry(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Where("id = ?", 7).Delete(&model.GStmtUser{})
	}
}

// ---------- B 组：真实往返（sqlite 内存库） ----------

func seedBoth(t testing.TB, db *opao.Database, gdb *gorm.DB) {
	for i := 0; i < 50; i++ {
		u := &stmtUser{Name: "u", Age: 10 + i}
		if err := db.Load(u).Create(); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 50; i++ {
		if err := gdb.Create(&model.GStmtUser{ID: int64(i + 1), Name: "u", Age: 10 + i}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkB_Find_Opao(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	o := db.Load(&stmtUser{ID: 5})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := o.Find("id = ?", 5); err != nil {
			b.Fatal(err)
		}
	}
	_ = gdb
}

func BenchmarkB_Find_Gorm(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	var out model.GStmtUser
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gdb.First(&out, 5).Error; err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}

func BenchmarkB_FindAllCond_Opao(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := db.Load(&stmtUser{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.FindAll(opao.Gt("age", 30)); err != nil {
			b.Fatal(err)
		}
	}
	_ = gdb
}

func BenchmarkB_FindAllCond_Gorm(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	var out []model.GStmtUser
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gdb.Where("age > ?", 30).Find(&out).Error; err != nil {
			b.Fatal(err)
		}
		out = out[:0]
	}
	_ = db
}

func BenchmarkB_Count_Opao(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := db.Load(&stmtUser{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.Count("age > ?", 30); err != nil {
			b.Fatal(err)
		}
	}
	_ = gdb
}

func BenchmarkB_Count_Gorm(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	var n int64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gdb.Model(&model.GStmtUser{}).Where("age > ?", 30).Count(&n).Error; err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}

func BenchmarkB_Create_Roundtrip_Opao(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u := &stmtUser{Name: "z", Age: 1}
		if err := db.Load(u).Create(); err != nil {
			b.Fatal(err)
		}
	}
	_ = gdb
}

func BenchmarkB_Create_Roundtrip_Gorm(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gdb.Create(&model.GStmtUser{Name: "z", Age: 1}).Error; err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}

var (
	_ = sql.ErrNoRows // 保持 import 使用
	_ = context.Background
)

// ---------- 引擎隔离实验：裸 database/sql，无任何 ORM 框架 ----------
// 验证 B 组 Count 差异来自底层引擎（mattn cgo vs modernc 纯Go）还是框架层。

func BenchmarkC_RawModern_Count(b *testing.B) {
	raw, err := sql.Open("sqlite", "file:bench_raw_modern?mode=memory&cache=shared")
	if err != nil {
		b.Fatal(err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`CREATE TABLE su (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT DEFAULT '', age INTEGER DEFAULT 0)`); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := raw.Exec("INSERT INTO su (name, age) VALUES ('u', ?)", 10+i); err != nil {
			b.Fatal(err)
		}
	}
	var n int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := raw.QueryRow("SELECT COUNT(*) FROM su WHERE age > ?", 30).Scan(&n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkC_RawMattn_Count(b *testing.B) {
	raw, err := sql.Open("sqlite3", "file:bench_raw_mattn?mode=memory&cache=shared")
	if err != nil {
		b.Fatal(err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`CREATE TABLE su (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT DEFAULT '', age INTEGER DEFAULT 0)`); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if _, err := raw.Exec("INSERT INTO su (name, age) VALUES ('u', ?)", 10+i); err != nil {
			b.Fatal(err)
		}
	}
	var n int
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := raw.QueryRow("SELECT COUNT(*) FROM su WHERE age > ?", 30).Scan(&n); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------- Gen（gorm.io/gen 代码生成层）三方对照 ----------

func newGenQ(t testing.TB, gdb *gorm.DB) *query.Query {
	return query.Use(gdb)
}

func BenchmarkA_Create_Gen(b *testing.B) {
	g := newGormDry(b)
	q := newGenQ(b, g)
	ctx := context.Background()
	rec := &model.GStmtUser{Name: "n", Age: 20}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = q.GStmtUser.WithContext(ctx).Create(rec)
	}
}

func BenchmarkA_Update_Gen(b *testing.B) {
	g := newGormDry(b)
	q := newGenQ(b, g)
	u := q.GStmtUser
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = u.WithContext(ctx).Where(u.ID.Eq(7)).Updates(map[string]any{"name": "x"})
	}
}

func BenchmarkA_Delete_Gen(b *testing.B) {
	g := newGormDry(b)
	q := newGenQ(b, g)
	u := q.GStmtUser
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = u.WithContext(ctx).Where(u.ID.Eq(7)).Delete()
	}
}

func BenchmarkB_Find_Gen(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := newGenQ(b, gdb)
	u := q.GStmtUser
	ctx := context.Background()
	var out model.GStmtUser
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := u.WithContext(ctx).Where(u.ID.Eq(5)).Take(); err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

func BenchmarkB_FindAllCond_Gen(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := newGenQ(b, gdb)
	u := q.GStmtUser
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := u.WithContext(ctx).Where(u.Age.Gt(30)).Find(); err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}

func BenchmarkB_Count_Gen(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := newGenQ(b, gdb)
	u := q.GStmtUser
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := u.WithContext(ctx).Where(u.Age.Gt(30)).Count(); err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}

func BenchmarkB_Create_Roundtrip_Gen(b *testing.B) {
	db, gdb := newRealPair(b)
	seedBoth(b, db, gdb)
	q := newGenQ(b, gdb)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := q.GStmtUser.WithContext(ctx).Create(&model.GStmtUser{Name: "z", Age: 1}); err != nil {
			b.Fatal(err)
		}
	}
	_ = db
}
