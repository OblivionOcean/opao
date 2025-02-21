package opao_test

import (
	"testing"

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
	_ "github.com/go-sql-driver/mysql"
)

//go test -benchmem -bench=^Benchmark -cpuprofile=cpu.pprof -memprofile=mem.pprof

func TestMysql(t *testing.T) {
	opao.NewDatabase("mysql", "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local")
}

func BenchmarkRegObj(t *testing.B) {
	// 测试Obj注册
	type TestObj struct {
		Id      int    `db:"id`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
	}
	orm := &support.ORM{}
	orm.Init(nil, nil)
	for i := 0; i < t.N; i++ {
		orm.Register("test", &TestObj{})
	}
}

func BenchmarkLoadObj(t *testing.B) {
	type TestObj struct {
		Id      int    `db:"id`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
	}
	orm := &support.ORM{}
	orm.Init(nil, mysql.NewMySQL)
	orm.Register("test", &TestObj{})
	for i := 0; i < t.N; i++ {
		orm.Load("test")
	}
}
