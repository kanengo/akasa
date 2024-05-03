package routing

import (
	"math"
	"slices"

	"github.com/kanengo/akasar/runtime/protos"
)

// EqualSlices 返回一个分配好的切片组.
// Replicas 以循环方式分配给多个切片 类似一致性哈希
func EqualSlices(replicas []string) *protos.Assignment {
	if len(replicas) == 0 {
		return &protos.Assignment{}
	}

	replicas = slices.Clone(replicas)
	slices.Sort(replicas)

	n := nextPowerOfTwo(len(replicas))
	slices := make([]*protos.Assignment_Slice, n)
	start := uint64(0)
	delta := math.MaxUint64 / uint64(n)
	for i := range n {
		slices[i] = &protos.Assignment_Slice{
			Start: start,
		}
		start += delta
	}

	// 循环分配replicas到slices
	for i, slice := range slices {
		slice.Replicas = []string{replicas[i%len(replicas)]}
	}

	return &protos.Assignment{
		Slices: slices,
	}
}

func nextPowerOfTwo(x int) int {
	switch {
	case x == 0:
		return 1
	case x&(x-1) == 0:
		return x
	default:
		return int(math.Pow(2, math.Ceil(math.Log2(float64(x)))))
	}
}
