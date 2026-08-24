# opao

[![GoDoc](https://pkg.go.dev/badge/github.com/OblivionOcean/opao)](https://pkg.go.dev/github.com/OblivionOcean/opao)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> 一个小巧、简单且高性能的 Go ORM 框架 🌟

## 项目简介

opao 是一款为性能而设计的轻量级 Go 语言 ORM（对象关系映射）框架。它采用了 `unsafe` + `cache` 的优化策略，在保证类型安全和线程安全的前提下，实现了卓越的数据库操作性能。

## 主要功能

- [x] **基础查询操作** - 单条记录查询、多条记录查询
- [x] **数据统计** - 记录总数统计
- [x] **数据更新** - 单字段、多字段更新
- [x] **数据删除** - 条件删除、批量删除
- [x] **数据插入** - 单条插入、批量插入
- [ ] 覆盖查询 - Upsert 操作
- [x] **查询条件生成** - 支持多种条件组合
- [x] **多数据库支持** - MySQL、PostgreSQL、SQLite3
- [x] **事务支持** - `db.Session(tx)` 注入事务；`WithContext` 透传 context
- [x] **GORM 风格链式查询** - `db.Model(&T{}).Where(...).Find(...)`
- [x] **GORM 模型迁移映射** - `RegisterSnake` 兼容 `gorm:` 标签与蛇形命名回退
- [x] **复用外部连接池** - `FromSQLDB` 与既有数据层共存
- [ ] 自动创建数据表
- [ ] 主从数据库支持
- [ ] 高级 SQL 功能（JOIN、子查询等）
- [ ] 数据库迁移工具

## 性能

与 GORM 及官方代码生成方案 GORM-Gen 同机同模型实测（构建组 DryRun；往返组 SQLite 内存库、同引擎 mattn）：

### 纯 SQL 构建

| 语句 | opao | GORM | GORM-Gen | vs Gen |
| ------ | ------ | ------ | ---------- | -------- |
| Create | **89ns / 1 alloc** | 7863ns / 61 | 8385ns / 68 | **94x** |
| Update | **154ns / 1 alloc** | 9362ns / 75 | 9831ns / 71 | **64x** |
| Delete | **159ns / 5 allocs** | 7611ns / 59 | 10399ns / 70 | **65x** |

### 真实往返（SQLite 内存库）

| 语句 | opao | GORM | GORM-Gen | vs Gen |
| ------ | ------ | ------ | ---------- | -------- |
| Find 单条 | **7.0µs** | 14.6µs | 13.3µs | 1.9x |
| FindAll 条件(50 行) | **32.9µs** | 45.6µs | 47.2µs | 1.4x |
| Count | **7.0µs** | 9.6µs | 12.2µs | 1.7x |
| Create 往返 | **5.3µs** | 22.3µs | 24.7µs | 4.7x |

> 复现：`cd dev/benchmarks && go test -bench=. -benchmem ./...`（方法学与引擎对齐说明见该目录 README；Gen 侧为生成代码直调，含 WithContext 与字段表达式开销）

### 完整数据
```shell
> cd dev/benchmarks && go test -run '^$' -bench 'Benchmark[AB]_' -benchtime=2s -benchmem .
goos: linux
goarch: amd64
pkg: github.com/OblivionOcean/opao/dev/benchmarks
cpu: 11th Gen Intel(R) Core(TM) i5-11300H @ 3.10GHz
BenchmarkA_Create_Opao-8              28416058         88.62 ns/op       16 B/op        1 allocs/op
BenchmarkA_Create_Gorm-8                324032       7863 ns/op     4683 B/op       61 allocs/op
BenchmarkA_Update_Opao-8              16636620        153.5 ns/op       16 B/op        1 allocs/op
BenchmarkA_Update_Gorm-8                243237       9362 ns/op     6107 B/op       75 allocs/op
BenchmarkA_Delete_Opao-8              18054232        158.8 ns/op      148 B/op        5 allocs/op
BenchmarkA_Delete_Gorm-8                330451       7611 ns/op     5115 B/op       59 allocs/op
BenchmarkB_Find_Opao-8                  338008       7036 ns/op      928 B/op       33 allocs/op
BenchmarkB_Find_Gorm-8                  161755      14578 ns/op     4514 B/op       74 allocs/op
BenchmarkB_FindAllCond_Opao-8            71437      32865 ns/op     4248 B/op      151 allocs/op
BenchmarkB_FindAllCond_Gorm-8            50883      45602 ns/op     6473 B/op      250 allocs/op
BenchmarkB_Count_Opao-8                 344094       7028 ns/op      752 B/op       21 allocs/op
BenchmarkB_Count_Gorm-8                 245964       9611 ns/op     3080 B/op       41 allocs/op
BenchmarkB_Create_Roundtrip_Opao-8      462164       5286 ns/op      808 B/op       16 allocs/op
BenchmarkB_Create_Roundtrip_Gorm-8      110958      22306 ns/op     5667 B/op       85 allocs/op
BenchmarkA_Create_Gen-8                 250809       8385 ns/op     5691 B/op       68 allocs/op
BenchmarkA_Update_Gen-8                 244675       9831 ns/op     6924 B/op       71 allocs/op
BenchmarkA_Delete_Gen-8                 263968      10399 ns/op     6420 B/op       70 allocs/op
BenchmarkB_Find_Gen-8                   160780      13337 ns/op     4519 B/op       64 allocs/op
BenchmarkB_FindAllCond_Gen-8             49344      47216 ns/op     8606 B/op      264 allocs/op
BenchmarkB_Count_Gen-8                  205082      12156 ns/op     6008 B/op       56 allocs/op
BenchmarkB_Create_Roundtrip_Gen-8       103322      24676 ns/op     6662 B/op       95 allocs/op
```

### 设计理念

- **高性能**：通过反射缓存和 unsafe 操作，最大程度减少运行时开销
- **类型安全**：编译时类型检查，避免运行时类型错误
- **线程安全**：内置并发控制机制，支持多协程安全访问
- **简单易用**：简洁的 API 设计，快速上手
- **零依赖**：无外部依赖，仅需引入对应的数据库驱动

## 安装指南

### 环境要求

- Go 1.21 或更高版本
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

# SQLite3 驱动（纯 Go）
go get modernc.org/sqlite
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
    Id   int64  `db:"id" option:"autoIncrement"`
    Name string `db:"name"`
    Age  int    `db:"age"`
}
```

兼容存量 GORM 模型：字段可改用 `gorm:"..."` 标签或省略标签由蛇形命名推断，用 `RegisterSnake` 注册即可。

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

// SQLite3 示例（modernc.org/sqlite 注册名 "sqlite"；mattn 为 "sqlite3"）
db, err := opao.New("sqlite", "./test.db")
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

// 插入数据，自增主键自动回填
err = objOrm.Create()
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
v, err := objOrm.Find("id = ?", 1)
if err != nil {
    panic(err)
}

// 使用条件查询
_, err = objOrm.Find("name = ?", "张三")

// 查询所有记录
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

// 按条件更新非零值字段（必须携带条件，否则返回 ErrMissingWhereClause）
err = objOrm.Update("id = ?", user.Id)

// Save 更新全部非自增字段(含零值)
err = objOrm.Save("id = ?", user.Id)

// 显式放行全表更新（慎用）
err = objOrm.Update(opao.AllowAll())
```

### 删除数据

```go
user := &User{}
objOrm := db.Load(user)

// 按条件删除（必须携带条件，否则返回 ErrMissingWhereClause）
err := objOrm.Delete("id = ?", 1)

// 批量删除
err = objOrm.Delete("age < ?", 18)
```

### 统计记录数

```go
user := &User{}
objOrm := db.Load(user)

// 条件统计
count, err := objOrm.Count("age > ?", 18)
fmt.Printf("符合条件的记录数: %d\n", count)
```

### 链式查询（GORM 风格）

```go
users, err := db.Model(&User{}).
    Where(opao.Gt("age", 18)).
    Order("-id").
    Limit(10).
    FindAll()
```

### 事务

```go
tx, err := db.Conn.Begin()
if err != nil {
    panic(err)
}
defer tx.Rollback() // 提交前回滚是安全默认

s := db.Session(tx)      // 该视图上的所有操作走 tx
u := &User{Name: "王五"}
if err := s.Load(u).Create(); err != nil {
    panic(err)
}
_ = s.Load(&User{}).Delete("id = ?", 1)

err = tx.Commit()
```

## 查询条件

opao 提供了丰富的查询条件构建函数：

```go
import (
    "github.com/OblivionOcean/opao"
)

// 等于
Eq("name", "张三")

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
    Id      int64  `db:"id" option:"autoIncrement"` // 自增主键
    Name    string `db:"name"`
    Age     int    `db:"age"`
    Created int64  `db:"created_time"`
    Email   string `db:"email"`
    Status  string `db:"status"`
    Private string `db:"-"` // 忽略该字段
}
```

### 可用的 option 选项

- `autoIncrement` - 标记为自增字段
- `-` - 忽略该字段（与 db 标签连用）

## 依赖项

opao 本身零外部依赖，只需安装对应数据库驱动：

| 数据库 | 驱动包 | 安装命令 |
|--------|--------|----------|
| MySQL | [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) | `go get github.com/go-sql-driver/mysql` |
| PostgreSQL | [lib/pq](https://github.com/lib/pq) | `go get github.com/lib/pq` |
| SQLite3 | [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)（纯 Go）| `go get modernc.org/sqlite` |

## 项目结构

```
opao/
├── condition.go            # 查询条件构建函数
├── db.go                   # 数据库连接管理与入口
├── query.go                # 链式查询（GORM 风格糖层）
├── internal/runtime/       # 运行时反射加速（含布局自检）
├── support/                # ORM 核心实现
│   ├── builder.go          # 共享 SQL 构建
│   ├── condition.go        # 条件渲染
│   ├── driver.go           # 共享 Driver 核心
│   ├── errors.go           # 哨兵错误
│   ├── exec.go             # 执行器抽象
│   ├── intern.go           # 语句/条件缓存
│   ├── orm.go              # Register/Load/SafeCache
│   ├── tags.go             # gorm 标签迁移映射
│   ├── mysql/              # MySQL 方言 + 驱动包装
│   ├── pg/                 # PostgreSQL 方言 + 驱动包装
│   └── sqlite/             # SQLite 方言 + 驱动包装
├── utils/                  # 通用工具
├── dev/                    # 测试与基准（嵌套模块）
│   ├── integration/        # 集成测试
│   └── benchmarks/         # 性能基准
├── go.mod
├── LICENSE                 # Apache 2.0 许可证
└── README.md               # 项目文档
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
cd dev/benchmarks && go test -bench=. -benchmem ./...

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

A: 通过 `Session` 将事务注入 ORM 视图：

```go
tx, _ := db.Conn.Begin()
defer tx.Rollback()

s := db.Session(tx)
s.Load(&User{Name: "x"}).Create()

tx.Commit()
```

### Q: 是否支持关联查询？

A: 目前暂不支持关联查询（JOIN）。您可以使用子查询或多次查询来实现关联数据的获取。

---

**注意**：opao 目前处于积极开发阶段，API 可能会有变动。
