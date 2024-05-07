package umath

import "math"

func FindNearestPow2(x int) int {
	x -= 1
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16

	if x < 0 {
		return 1
	}

	return x + 1
}

func NextPowerOfTwo(x int) int {
	switch {
	case x == 0:
		return 1
	case x&(x-1) == 0:
		return x
	default:
		return int(math.Pow(2, math.Ceil(math.Log2(float64(x)))))
	}
}
