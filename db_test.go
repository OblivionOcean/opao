package opao_test
/*
import (
	"testing"

	"github.com/OblivionOcean/opao"
	"github.com/OblivionOcean/opao/support"
	"github.com/OblivionOcean/opao/support/mysql"
	_ "github.com/go-sql-driver/mysql"
	//"gorm.io/driver/sqlite" // 使用SQLite内存数据库进行隔离测试
	//"gorm.io/gorm"
)

//go test -benchmem -bench=^Benchmark -cpuprofile=cpu.pprof -memprofile=mem.pprof

func TestMysql(t *testing.T) {
	opao.NewDatabase("mysql", "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local")
}

func BenchmarkOpaoRegObj(b *testing.B) {
	// 测试Obj注册
	type TestObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
	}
	orm := &support.ORM{}
	orm.Init(nil, nil)
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		orm.Register("test", &TestObj{})
	}
}

func BenchmarkOpaoLoadObj(b *testing.B) {
	type TestObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
	}
	orm := &support.ORM{}
	orm.Init(nil, mysql.NewMySQL)
	orm.Register("test", &TestObj{})
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		orm.Load(&TestObj{})
	}
}

func BenchmarkOpaoUpdateObj(b *testing.B) {
	type TestObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
		blob    []byte `db:"blob"`
	}
	orm := &support.ORM{}
	orm.Init(nil, mysql.NewMySQL)
	orm.Register("test", &TestObj{})
	o := orm.Load(&TestObj{
		ID: 11,
	})
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		o.Update()
	}
}

func BenchmarkOpaoSaveObj(b *testing.B) {
	type TestObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
		blob    []byte `db:"blob"`
	}
	orm := &support.ORM{}
	orm.Init(nil, mysql.NewMySQL)
	orm.Register("test", &TestObj{})
	o := orm.Load(&TestObj{
		ID: 11,
	})
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		o.Update()
	}
}

func TestPgSql(t *testing.T) {
	type TestObj struct {
		ID      int    `db:"id"`
		Name    string `db:"name"`
		UserAge int    `db:"user_age"`
		my      int    `db:"my"`
	}
	orm := &support.ORM{}
	orm.Init(nil, mysql.NewMySQL)
	orm.Register("test", &TestObj{})
	tmp := orm.Load(&TestObj{})
	tmp.Update("\"url\" = ?", "")
}

/*

func BenchmarkGormSave(b *testing.B) {
	// 初始化GORM连接，这里使用SQLite内存模式避免网络开销
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.Session(&gorm.Session{DryRun: true})
	// 定义您的模型，结构需要与您的SQL示例匹配
	type Test struct {
		ID      uint
		Name    string
		UserAge int
		My      string
	}
	db.AutoMigrate(&Test{}) // 创建表结构
	t := &Test{}
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		// 此操作包含了SQL生成、驱动处理等，但主要在内存中完成，可近似看作生成耗时
		db.Model(t).Where("").Save(t)
	}
}

func BenchmarkGormUpdate(b *testing.B) {
	// 初始化GORM连接，这里使用SQLite内存模式避免网络开销
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.Session(&gorm.Session{DryRun: true})
	// 定义您的模型，结构需要与您的SQL示例匹配
	type Test struct {
		ID      uint
		Name    string
		UserAge int
		My      string
	}
	db.AutoMigrate(&Test{}) // 创建表结构
	t := &Test{
		ID: 14,
	}
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		// 此操作包含了SQL生成、驱动处理等，但主要在内存中完成，可近似看作生成耗时
		db.Model(t).Where("").UpdateColumns(t)
	}
}


func BenchmarkGormRegObj(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.Session(&gorm.Session{DryRun: true})
	// 定义您的模型，结构需要与您的SQL示例匹配
	type Test struct {
		ID      uint
		Name    string
		UserAge int
		My      string
	}
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		db.AutoMigrate(&Test{}) // 创建表结构
	}
}

func BenchmarkGormLoadObj(b *testing.B) {
	// 初始化GORM连接，这里使用SQLite内存模式避免网络开销
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.Session(&gorm.Session{DryRun: true})
	// 定义您的模型，结构需要与您的SQL示例匹配
	type Test struct {
		ID      uint
		Name    string
		UserAge int
		My      string
	}
	db.AutoMigrate(&Test{}) // 创建表结构
	t := &Test{}
	b.ResetTimer() // 重置计时器，排除初始化耗时
	for i := 0; i < b.N; i++ {
		// 此操作包含了SQL生成、驱动处理等，但主要在内存中完成，可近似看作生成耗时
		db.Model(t)
	}
}
*/
