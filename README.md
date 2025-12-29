# opao
[![GoDoc](https://pkg.go.dev/badge/github.com/OblivionOcean/opao)](https://pkg.go.dev/github.com/OblivionOcean/opao)
[![Go Report Card](https://goreportcard.com/badge/github.com/OblivionOcean/opao)](https://goreportcard.com/report/github.com/OblivionOcean/opao)
一个小巧，简单的ORM🌟A small, simple ORM
为了性能，它使用了`unsafe`+`cache`，同时保证类型安全和线程安全，尽可能的做到了最好。
同时有很好的兼容性，可以运行在主流操作系统中，并且支持`go1.20+`
它集成了基础的`MySQL`、`PgSQL`、`SQLite3`
## 安装
```shell
go get github.com/OblivionOcean/opao
```

## 功能
- [x] 基础的查询
- [x] 基础的总数统计
- [x] 基础的更新
- [x] 基础的删除
- [x] 基础的插入
- [x] 基础的覆盖查询
- [x] 查询条件生成
- [x] 查询条件生成
- [ ] 创建数据表
- [ ] 主从数据库支持
- [ ] 高级SQL功能
- [ ] ...

## 基准测试

> 本测试仅为SQL语句生成，不涉及实际数据库交互。测试结果不作为生产环境参考值。测试的Gorm和opao均为为Classic版本，不是Gen版本。参与测试的Gorm为v1.31.1版本。

```bash
> go test -benchmem -bench=^Benchmark -v -cpuprofile ./cpu.pprof
=== RUN   TestMysql
--- PASS: TestMysql (0.00s)
=== RUN   TestPgSql
--- PASS: TestPgSql (0.00s)
goos: linux
goarch: amd64
pkg: github.com/OblivionOcean/opao
cpu: 11th Gen Intel(R) Core(TM) i5-11300H @ 3.10GHz
BenchmarkOpaoRegObj
BenchmarkOpaoRegObj-8            2681996               445.6 ns/op           448 B/op          5 allocs/op
BenchmarkOpaoLoadObj
BenchmarkOpaoLoadObj-8          11621361                99.50 ns/op          144 B/op          2 allocs/op
BenchmarkOpaoUpdateObj
BenchmarkOpaoUpdateObj-8        11765991               101.5 ns/op            96 B/op          2 allocs/op
BenchmarkOpaoSaveObj
BenchmarkOpaoSaveObj-8          11492600               100.7 ns/op            96 B/op          2 allocs/op
BenchmarkGormSave
BenchmarkGormSave-8                74766             15450 ns/op            7534 B/op         92 allocs/op
BenchmarkGormUpdate
BenchmarkGormUpdate-8             165680              6935 ns/op            4491 B/op         54 allocs/op
BenchmarkGormRegObj
BenchmarkGormRegObj-8              17763             67401 ns/op           35726 B/op        591 allocs/op
BenchmarkGormLoadObj
BenchmarkGormLoadObj-8           5153940               232.6 ns/op           704 B/op          4 allocs/op
PASS
ok      github.com/OblivionOcean/opao   11.530s
```

## 使用
```go
package main

import (
	"github.com/OblivionOcean/opao"
)

type User struct {
	Id   int64  `db:"id"`
	Name string `db:"name"`
    hi   string `db:"hi"`// 支持未导出的字段
}

func main() {
	db, err := opao.New("mysql", "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err != nil {
		panic(err)
	}
    db.Register("user", &User{})// 前者是数据表名
	// 插入数据
	user := &User{
		Name: "test",
	}
    objOrm := db.Load(user)
	err = objOrm.Create(user)
	if err != nil {
		panic(err)
	}
}
```
