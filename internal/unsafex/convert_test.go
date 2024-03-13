package unsafex

import (
	"fmt"
	"testing"
)

func TestStringToBytes(t *testing.T) {
	s := "hello world"
	fmt.Println(BytesToString(StringToBytes(s)))
}
