# opao

[![GoDoc](https://pkg.go.dev/badge/github.com/OblivionOcean/opao)](https://pkg.go.dev/github.com/OblivionOcean/opao)
[![Go Report Card](https://goreportcard.com/badge/github.com/OblivionOcean/opao)](https://goreportcard.com/report/github.com/OblivionOcean/opao)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> 一个小巧、简单且高性能的 Go ORM 框架 🌟

## 项目简介

opao 是一款为性能而设计的轻量级 Go 语言 ORM（对象关系映射）框架。它采用了 `unsafe` + `cache` 的优化策略，在保证类型安全和线程安全的前提下，实现了卓越的数据库操作性能。

### 设计理念

- **高性能**：通过反射缓存和 unsafe 操作，最大程度减少运行时开销
- **类型安全**：编译时类型检查，避免运行时类型错误
- **线程安全**：内置并发控制机制，支持多协程安全访问
- **简单易用**：简洁的 API 设计，快速上手
- **零依赖**：无外部依赖，仅需引入对应的数据库驱动

## 主要功能

- [x] **基础查询操作** - 单条记录查询、多条记录查询
- [x] **数据统计** - 记录总数统计
- [x] **数据更新** - 单字段、多字段更新
- [x] **数据删除** - 条件删除、批量删除
- [x] **数据插入** - 单条插入、批量插入
- [x] **覆盖查询** - Upsert 操作
- [x] **查询条件生成** - 支持多种条件组合
- [x] **多数据库支持** - MySQL、PostgreSQL、SQLite3
- [ ] 自动创建数据表
- [ ] 主从数据库支持
- [ ] 高级 SQL 功能（JOIN、子查询等）
- [ ] 数据库迁移工具
- [ ] 连接池管理增强
- [ ] 查询结果缓存

## 安装指南

### 环境要求

- Go 1.20 或更高版本
- 操作系统：Linux、macOS、Windows

### 安装步骤

```bash
go get github.com/OblivionOcean/opao
```

### 安装数据库驱动

根据您使用的数据库类型，安装对应的驱动：

```bash
# MySQL 驱动
go get github.com/go-sql-driver/mysql

# PostgreSQL 驱动
go get github.com/lib/pq

# SQLite3 驱动
go get github.com/mattn/go-sqlite3
```

## 使用说明

### 快速开始

#### 1. 定义数据模型

```go
package main

import (
    "github.com/OblivionOcean/opao"
)

type User struct {
    Id   int64  `db:"id"`
    Name string `db:"name"`
    Age  int    `db:"age"`
    // 支持未导出字段
    hi   string `db:"hi"`
}
```

#### 2. 初始化数据库连接

```go
// MySQL 示例
db, err := opao.New("mysql", "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local")
if err != nil {
    panic(err)
}
defer db.Close()

// PostgreSQL 示例
db, err := opao.New("postgres", "user=postgres password=123456 dbname=test host=127.0.0.1 port=5432 sslmode=disable")

// SQLite3 示例
db, err := opao.New("sqlite3", "./test.db")
```

#### 3. 注册模型

```go
// 注册 User 模型，第一个参数为数据表名
err := db.Register("user", &User{})
if err != nil {
    panic(err)
}
```

### 插入数据

```go
user := &User{
    Name: "张三",
    Age:  25,
}

// 创建 ORM 对象
objOrm := db.Load(user)

// 插入数据
err = objOrm.Create(user)
if err != nil {
    panic(err)
}
```

### 查询数据

```go
// 查询单条记录
user := &User{}
objOrm := db.Load(user)

// 使用主键查询
err = objOrm.Find("id = ?", 1)
if err != nil {
    panic(err)
}

// 使用条件查询
err = objOrm.Find("name = ?", "张三")

// 查询所有记录
var users []*User
results, err := objOrm.FindAll("age > ?", 18)
for _, v := range results {
    user := v.(*User)
    fmt.Printf("ID: %d, Name: %s\n", user.Id, user.Name)
}
```

### 更新数据

```go
user := &User{Id: 1, Name: "李四"}
objOrm := db.Load(user)

// 更新所有字段
err = objOrm.Update()

// 更新指定字段
err = objOrm.Update("name = ?, age = ?", "王五", 30)

// 使用条件更新
err = objOrm.Update("id = ?", 1)
```

### 删除数据

```go
user := &User{}
objOrm := db.Load(user)

// 删除指定条件的数据
err = objOrm.Delete("id = ?", 1)

// 批量删除
err = objOrm.Delete("age < ?", 18)
```

### 统计记录数

```go
user := &User{}
objOrm := db.Load(user)

// 统计总数
count, err := objOrm.Count()

// 条件统计
count, err := objOrm.Count("age > ?", 18)
fmt.Printf("符合条件的记录数: %d\n", count)
```

## 查询条件

opao 提供了丰富的查询条件构建函数：

```go
import (
    "github.com/OblivionOcean/opao"
)

// 等于
Eq("name", "张三")

// 不等于
NotEq("name", "张三")

// 大于
Gt("age", 18)

// 小于
Lt("age", 60)

// 大于等于
Gte("age", 18)

// 小于等于
Lte("age", 60)

// IN 条件
In("id", []any{1, 2, 3})

// LIKE 模糊查询
Like("name", "%张%")

// NOT LIKE
NotLike("name", "%李%")

// BETWEEN 范围查询
Between("age", 18, 30)

// NOT BETWEEN
NotBetween("age", 18, 30)

// EXISTS
Exists(opao.Custom("SELECT 1 FROM orders WHERE user_id = user.id"))

// NOT EXISTS
NotExists(opao.Custom("SELECT 1 FROM orders WHERE user_id = user.id"))

// AND 条件组合
And(
    Eq("name", "张三"),
    Gt("age", 18),
)

// OR 条件组合
Or(
    Eq("name", "张三"),
    Eq("name", "李四"),
)

// NOT 条件
Not(Eq("age", 18))

// 自定义条件
Custom("JSON_EXTRACT(data, '$.key') = ?", "value")

// 子查询
InSubquery("id", "SELECT id FROM active_users")

// 限制结果数量
Limit(10)

// 限制结果数量并偏移
LimitOffset(10, 20) // LIMIT 10 OFFSET 20
```

### 条件组合示例

```go
// 复杂条件查询
conditions := And(
    Or(
        Eq("status", "active"),
        Eq("status", "pending"),
    ),
    Gte("age", 18),
    Not(
        In("id", []any{1, 2, 3}),
    ),
)

results, err := objOrm.FindAll(conditions)
```

## 配置选项

### 数据模型标签

opao 使用 `db` 标签来映射结构体字段到数据库列：

```go
type User struct {
    Id      int64  `db:"id"`
    Name    string `db:"name"`
    Age     int    `db:"age"`
    Created int64  `db:"created_time"`
    Email   string `db:"email"`
    Status  string `db:"status"`

    // 使用 option 标签配置额外选项
    Id      int64  `db:"id" option:"autoIncrement"` // 自增主键
    Private string `db:"-"`                          // 忽略该字段
}
```

### 可用的 option 选项

- `autoIncrement` - 标记为自增字段
- `-` - 忽略该字段（与 db 标签连用）

## 性能基准测试

### 测试环境

- 操作系统：Linux
- 架构：amd64
- CPU：11th Gen Intel(R) Core(TM) i5-11300H @ 3.10GHz
- Go 版本：1.21+

### 基准测试结果

> **说明**：本测试仅测量 SQL 语句生成的性能，不涉及实际数据库交互。测试结果不作为生产环境参考值。参与测试的 Gorm 为 v1.31.1 版本。

```bash
go test -benchmem -bench=^Benchmark -v -cpuprofile=./cpu.pprof

=== RUN   TestMysql
--- PASS: TestMysql (0.00s)
=== RUN   TestPgSql
--- PASS: TestPgSql (0.00s)

goos: linux
goarch: amd64
pkg: github.com/OblivionOcean/opao
cpu: 11th Gen Intel(R) Core(TM) i5-11300H @ 3.10GHz

BenchmarkOpaoRegObj-8        2681996    445.6 ns/op    448 B/op    5 allocs/op
BenchmarkOpaoLoadObj-8      11621361     99.50 ns/op    144 B/op    2 allocs/op
BenchmarkOpaoUpdateObj-8    11765991    101.5 ns/op      96 B/op    2 allocs/op
BenchmarkOpaoSaveObj-8     11492600    100.7 ns/op      96 B/op    2 allocs/op
BenchmarkGormSave-8           74766  15450 ns/op      7534 B/op   92 allocs/op
BenchmarkGormUpdate-8       165680   6935 ns/op      4491 B/op   54 allocs/op
BenchmarkGormRegObj-8        17763  67401 ns/op     35726 B/op  591 allocs/op
BenchmarkGormLoadObj-8     5153940    232.6 ns/op      704 B/op    4 allocs/op

PASS
ok      github.com/OblivionOcean/opao   11.530s
```

### 性能优势

从基准测试可以看出，opao 在各项指标上均显著优于 Gorm：

- **注册性能**：约 151 倍于 Gorm
- **加载性能**：约 2.3 倍于 Gorm
- **更新性能**：约 68 倍于 Gorm
- **保存性能**：约 153 倍于 Gorm
- **内存分配**：显著更少的内存分配

## 依赖项

opao 本身零外部依赖，只需安装对应数据库驱动：

| 数据库 | 驱动包 | 安装命令 |
|--------|--------|----------|
| MySQL | [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) | `go get github.com/go-sql-driver/mysql` |
| PostgreSQL | [lib/pq](https://github.com/lib/pq) | `go get github.com/lib/pq` |
| SQLite3 | [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | `go get github.com/mattn/go-sqlite3` |

## 项目结构

```
opao/
├── condition.go          # 查询条件构建函数
├── db.go                 # 数据库连接管理
├── db_test.go            # 基准测试
├── internal/              # 内部工具包
│   └── runtime/          # 运行时反射相关
├── support/              # ORM 核心实现
│   ├── condition.go      # 条件定义
│   ├── elem.go           # 元素操作
│   ├── orm.go            # ORM 接口实现
│   ├── mysql/            # MySQL 支持
│   ├── pg/               # PostgreSQL 支持
│   ├── sqlite/           # SQLite 支持
│   └── utils/            # 工具函数
├── utils/                # 通用工具
├── go.mod                # Go 模块定义
├── go.sum                # 依赖版本锁定
├── LICENSE               # Apache 2.0 许可证
└── README.md             # 项目文档
```

## 贡献指南

我们欢迎任何形式的贡献！如果您想为 opao 做出贡献，请遵循以下步骤：

### 提交问题

如果您发现 bug 或有功能建议，请在 GitHub Issues 中提交：

1. 清晰描述问题或需求
2. 提供复现步骤（如果是 bug）
3. 附上相关代码示例
4. 说明您期望的行为

### 提交代码

1. **Fork 本仓库**
2. **创建特性分支** (`git checkout -b feature/AmazingFeature`)
3. **提交更改** (`git commit -m 'Add some AmazingFeature'`)
4. **推送到分支** (`git push origin feature/AmazingFeature`)
5. **开启 Pull Request**

### 代码规范

- 遵循 Go 语言官方代码规范
- 添加必要的注释和文档
- 确保所有测试通过 (`go test ./...`)
- 运行 `go fmt` 格式化代码
- 添加或更新测试用例

### 开发环境

```bash
# 克隆仓库
git clone https://github.com/OblivionOcean/opao.git
cd opao

# 运行测试
go test ./...

# 运行基准测试
go test -bench=. -benchmem

# 运行代码检查
go vet ./...
```

## 许可证

本项目采用 Apache License 2.0 开源许可证。

```
Copyright 2024 OblivionOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

## 联系方式

- **作者**：OblivionOcean
- **项目地址**：[https://github.com/OblivionOcean/opao](https://github.com/OblivionOcean/opao)
- **文档地址**：[https://pkg.go.dev/github.com/OblivionOcean/opao](https://pkg.go.dev/github.com/OblivionOcean/opao)
- **问题反馈**：[GitHub Issues](https://github.com/OblivionOcean/opao/issues)

## 常见问题 (FAQ)

### Q: opao 与其他 ORM 框架相比有什么优势？

A: opao 的主要优势在于：

- **性能优异**：通过反射缓存和 unsafe 操作，性能显著优于主流 ORM
- **零依赖**：不依赖任何第三方库，仅需要数据库驱动
- **简单易用**：API 设计简洁，学习成本低
- **类型安全**：编译时类型检查，减少运行时错误

### Q: opao 支持哪些数据库？

A: 目前支持 MySQL、PostgreSQL 和 SQLite3。未来计划支持更多数据库。

### Q: 如何处理事务？

A: opao 使用标准库 `database/sql`，可以直接使用其事务功能：

```go
tx, err := db.Conn.Begin()
if err != nil {
    panic(err)
}
defer tx.Rollback()

// 执行操作
objOrm := db.Load(&User{})
// ...

err = tx.Commit()
```

### Q: 是否支持关联查询？

A: 目前暂不支持关联查询（JOIN），这是计划中的功能。您可以使用子查询或多次查询来实现关联数据的获取。

## 致谢

感谢所有为 opao 做出贡献的开发者！

---

**注意**：opao 目前处于积极开发阶段，API 可能会有变动。