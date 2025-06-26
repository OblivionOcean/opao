# opao
一个小巧，简单的ORM🌟A small, simple ORM
为了性能，它使用了很多`unsafe`，尽可能的做到了最好。
同时有很好的兼容性，可以运行在主流操作系统中，并且支持`go1.20+`
它集成了基础的`Mysql`、`Pg`、`Sqlite`
## 安装
```shell
go get github.com/OblivionOcean/opao
```

## 功能
* [x] 基础的查询
- [x] 基础的更新
- [x] 基础的删除
- [x] 基础的插入
- [x] 基础的覆盖查询
- [x] 查询条件生成
- [ ] 创建数据表
- [ ] 主从数据库支持
- [ ] 高级SQL功能
- [ ] ...

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