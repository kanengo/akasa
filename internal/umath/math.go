package umath

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
