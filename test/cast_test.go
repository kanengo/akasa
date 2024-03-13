package test

import (
	"fmt"
	"testing"
)

func TestUnsigned(t *testing.T) {
	i32 := int32(-10)
	fmt.Println(i32)
	u32 := uint32(i32)
	fmt.Println(u32)
	fmt.Println(int32(u32))
}
