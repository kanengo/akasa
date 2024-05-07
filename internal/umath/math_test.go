package umath

import (
	"fmt"
	"testing"
)

func TestFindNearestPow2(t *testing.T) {
	x := 123456789
	fmt.Println(FindNearestPow2(x))
}

func foo() *[]byte {
	s := make([]byte, 0, 128)
	fmt.Println(fmt.Sprintf("init %p", &s))
	return &s
}

func TestNextPowerOfTwo(t *testing.T) {
	fmt.Println(NextPowerOfTwo(128))
	for _, x := range []int{
		1, 3, 15, 26, 56, 89, 170, 300, 601, 1400, 3024, 7034, 12012, 24000, 40232, 124141412, 53253253, 23532532532,
	} {
		if NextPowerOfTwo(x) != FindNearestPow2(x) {
			t.Error("not match")
		}

	}
}
