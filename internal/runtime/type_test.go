package runtime_test

import (
	"reflect"
	"testing"

	"github.com/OblivionOcean/opao/internal/runtime"
)

func BenchmarkType(b *testing.B) {
	test := struct {
		Name string `json:"name"`
	}{
		Name: "test",
	}
	for i := 0; i < b.N; i++ {
		sf := &reflect.StructField{}
		runtime.GetField(sf, runtime.Type2StructType(reflect.TypeOf(test)), 0)
		//b.Log(sf)
		Used(sf)
	}
}

func Used(...any) {}
