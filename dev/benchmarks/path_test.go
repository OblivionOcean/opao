// 复刻旧 README 基准语义（仅 ID 非零、无 autoIncrement 标记、无条件），
// 与现行"带条件构建"分别测量，澄清 101ns 旧数字的可比性。
// 考古结论：旧基准在 commit 376ecdd 实测 panic（nil *sql.DB 上 Exec），
// README 的 101.5ns 从未是有效的全路径测量。
package benchmarks

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"unsafe"

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
)

func TestPathBreakdown(t *testing.T) {
	type oldBenchObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
	}
	obj := &oldBenchObj{ID: 11}
	orm := &support.ORM{}
	orm.Init(nil, mysql.New)
	if err := orm.Register("test", &oldBenchObj{}); err != nil {
		t.Fatal(err)
	}
	_, elems, typ, err := orm.Snapshot(obj)
	if err != nil {
		t.Fatal(err)
	}
	base := reflect.ValueOf(obj).UnsafePointer()
	for i := range elems {
		elems[i].Ptr = unsafe.Add(base, elems[i].Offset)
	}

	b1 := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, _, err := support.BuildUpdate(mysql.Dialect, "test", elems, false, obj, typ); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("今·无条件全路径构建(等价旧基准意图): %s", b1.String())

	b2 := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, _, err := support.BuildUpdate(mysql.Dialect, "test", elems, false, obj, typ, "id = ?", 11); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("今·带 WHERE 条件构建(现行基准形态): %s", b2.String())
}

type driverRows int64

func (d driverRows) LastInsertId() (int64, error) { return int64(d), nil }
func (d driverRows) RowsAffected() (int64, error) { return int64(d), nil }

// fakeExecer 无 I/O 执行器：隔离出驱动层构建+池化的纯 CPU 成本
type fakeExecer struct{}

func (fakeExecer) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return driverRows(1), nil
}
func (fakeExecer) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return nil, sql.ErrNoRows
}
func (fakeExecer) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row { return nil }

// BenchmarkOpao_DriverUpdate 驱动级 Update 热路径：
// 位模式 SET 缓存 + L2 条件缓存 + 池化 buffer/args，零稳态分配目标
func BenchmarkOpao_DriverUpdate(b *testing.B) {
	type row struct {
		ID   int64  `db:"id" option:"autoIncrement"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	obj := &row{ID: 11, Name: "x"}
	orm := &support.ORM{}
	orm.Init(nil, mysql.New)
	orm.Register("bench_t", &row{})
	o := orm.Load(obj).(*mysql.MySQL)
	o.BindExec(fakeExecer{}, context.Background())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := o.Update("id = ?", 11); err != nil {
			b.Fatal(err)
		}
	}
}

// 缓存层审计：渲染直出 vs L2 缓存路径，验证每层是否真赚
func BenchmarkAudit_RenderDirect(b *testing.B) {
	type row struct {
		ID   int64  `db:"id" option:"autoIncrement"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	obj := &row{ID: 11, Name: "x", Age: 30}
	orm := &support.ORM{}
	orm.Init(nil, mysql.New)
	orm.Register("audit_t", &row{})
	_, elems, typ, _ := orm.Snapshot(obj)
	c := opao.Eq("age", 30)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := support.RenderCondition(elems, obj, typ, c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAudit_CachedRender(b *testing.B) {
	type row struct {
		ID   int64  `db:"id" option:"autoIncrement"`
		Name string `db:"name"`
		Age  int    `db:"age"`
	}
	obj := &row{ID: 11, Name: "x", Age: 30}
	orm := &support.ORM{}
	orm.Init(nil, mysql.New)
	orm.Register("audit_t2", &row{})
	_, elems, typ, _ := orm.Snapshot(obj)
	c := opao.Eq("age", 30)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := support.CachedRender(mysql.Dialect, "audit_t2", elems, obj, typ, c); err != nil {
			b.Fatal(err)
		}
	}
}
