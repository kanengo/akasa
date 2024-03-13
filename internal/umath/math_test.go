package umath

import (
	"fmt"
	"testing"
)

func TestFindNearestPow2(t *testing.T) {
	x := 123456789
	fmt.Println(FindNearestPow2(x))
}
