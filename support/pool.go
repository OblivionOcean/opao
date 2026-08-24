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

import (
	"sync"

	"github.com/OblivionOcean/opao/utils"
)

// 热路径对象池：驱动层在 Exec 完成后归还，消除稳态分配。
// 注意：经池化的切片内容在归还后失效——调用方不得长期持有。

const argsPoolCap = 32

var (
	argsPool = sync.Pool{New: func() any {
		s := make([]any, 0, argsPoolCap)
		return &s
	}}
	bufPool = sync.Pool{New: func() any {
		b := utils.NewBuffer(256)
		return &b
	}}
)

func getArgs() *[]any        { return argsPool.Get().(*[]any) }
func putArgs(a *[]any)       { *a = (*a)[:0]; argsPool.Put(a) }
func getBuf() *utils.Buffer  { return bufPool.Get().(*utils.Buffer) }
func putBuf(b *utils.Buffer) { *b = (*b)[:0]; bufPool.Put(b) }

// GetPooledArgs / PutPooledArgs 导出的池化绑定值切片（糖层使用）。
func GetPooledArgs() *[]any  { return getArgs() }
func PutPooledArgs(a *[]any) { putArgs(a) }

// GetPooledBuf / PutPooledBuf 导出的池化缓冲。
func GetPooledBuf() *utils.Buffer  { return getBuf() }
func PutPooledBuf(b *utils.Buffer) { putBuf(b) }
