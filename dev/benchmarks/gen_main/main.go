// 一次性生成 gen 的类型安全查询包（产物提交进仓库，测试直接引用）
package main

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gen"
	"gorm.io/gorm"

	"github.com/OblivionOcean/opao/dev/benchmarks/model"
)

func main() {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := gdb.AutoMigrate(&model.GStmtUser{}); err != nil {
		panic(err)
	}
	g := gen.NewGenerator(gen.Config{
		OutPath: "./query",
		Mode:    gen.WithDefaultQuery,
	})
	g.UseDB(gdb)
	g.ApplyBasic(&model.GStmtUser{})
	g.Execute()
}
