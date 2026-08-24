// Copyright 2024 OblivionOcean
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package support

// Dialect 数据库方言抽象，封装标识符引用与占位符差异。
// 各方言实现位于本目录的子包：support/mysql、support/pg、support/sqlite。
type Dialect interface {
	// Name 返回方言注册名："mysql" / "postgres" / "sqlite"
	Name() string
	// Quote 引用 SQL 标识符（列名/表名）
	Quote(identifier string) string
	// BindVar 返回第 n 个绑定参数占位符，n 从 1 开始
	BindVar(n int) string
}
