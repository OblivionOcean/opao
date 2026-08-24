# opao 基准测试（vs GORM）

对 opao 与 GORM 做同模型、同数据集的公平对比。两组基准分开报告，**不要跨组比较**。

## 运行

```bash
cd dev/benchmarks
go test -run '^$' -bench=. -benchmem ./...
```

只跑某一组：

```bash
# 组 A：纯 SQL 构建（不触库）
go test -run '^$' -bench='BuildUpdate|DryRun' -benchmem ./...

# 组 B：真实往返（sqlite 内存库）
go test -run '^$' -bench='Roundtrip' -benchmem ./...
```

## 环境变量说明

- `GOPROXY`：本机已配置 `https://goproxy.cn,direct`；首次构建会拉取 gorm 与 sqlite 驱动依赖。
- `CGO_ENABLED`：默认使用 `gorm.io/driver/sqlite`（基于 cgo）。若环境无 C 工具链导致编译失败，
  可将 `bench_test.go` 中 gorm 侧驱动替换为纯 Go 的 `github.com/glebarez/sqlite`
  （API 兼容，同样是 `sqlite.Open(":memory:")`），并重新执行 `go mod tidy`。
- `GOMAXPROCS`：基准含真实 DB 往返，建议固定（如 `GOMAXPROCS=4`）以降低结果抖动。
- `GODEBUG=cgocheck=0` 等运行时调试项不建议开启，会影响分配数统计的准确性。

## 分组与方法论

**组 A：纯 SQL 构建（DryRun 类）**
- `BenchmarkOpao_BuildUpdate`：直接调用 `support.BuildUpdate(DialectMySQL, ...)` 的构建路径，
  元数据来自 Register 后的 `ORM.Snapshot`，不触库。
- `BenchmarkGorm_DryRunUpdate`：gorm `Session{DryRun: true}` 只生成 SQL 不执行。

衡量的是"把对象/条件变成 SQL"这一层的开销。

**组 B：真实往返（sqlite 内存库，单连接）**

两侧均为内存 SQLite、`SetMaxOpenConns(1)`、建表后预插 100 行，`b.ResetTimer()` 排除初始化：
- `BenchmarkOpao_Roundtrip_FindAll` / `BenchmarkGorm_Roundtrip_FindAll`：条件 `age > ?` 全行返回，
  并断言行数防止编译器/驱动短路优化。
- `BenchmarkOpao_Roundtrip_UpdateByID` / `BenchmarkGorm_Roundtrip_UpdateByID`：
  按主键轮换更新 name 列（opao 跳过零值列与自增列，gorm 单列 Update，语义一致）。

## 诚实声明

所有结果都依赖本机 CPU、内存、Go 版本、sqlite 驱动实现（cgo vs 纯 Go）等环境因素，
**仅作相对参考，不构成任何普适性结论**。请在你自己的目标硬件与数据库上复测后再下判断。

## 引擎公平性说明（重要）

往返组(B组)两侧统一使用 mattn/go-sqlite3(cgo) 引擎：opao 经
`NewDatabase("sqlite3", ...)` 接入，gorm 经 `gorm.io/driver/sqlite`。
若 opao 使用 modernc 纯 Go 引擎，单语句会慢约 70%（引擎差异，
非框架差异——C 组裸 database/sql 对照可验证）。

框架层开销对照（B_Count 减 C_Raw 同引擎基线）：
opao ≈1.2µs vs gorm ≈4.0µs。
