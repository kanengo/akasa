package test

import (
	"fmt"
	"reflect"
	"testing"
)

func TestPanicType(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				fmt.Printf("error:%s  %s\n", e.Error(), reflect.TypeOf(e).String())
			} else {
				fmt.Printf("panic:%v\n", r)
			}
		}
	}()

	a := 10
	b := 0
	z := a / b
	_ = z
	//panic(fmt.Errorf("panic test"))
	//var a = []int{1, 0}
	//var b = a[10]
	//_ = b
	//var m map[string]string = nil
	//
	//m["a"] = "a"
}
