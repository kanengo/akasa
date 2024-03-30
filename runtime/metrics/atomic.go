package metrics

import (
	"math"
	"sync/atomic"
)

type atomicFloat64 struct {
	v atomic.Uint64
}

func (f *atomicFloat64) get() float64 {
	return math.Float64frombits(f.v.Load())
}

func (f *atomicFloat64) set(v float64) {
	f.v.Store(math.Float64bits(v))
}

func (f *atomicFloat64) add(v float64) {
	for {
		cur := f.v.Load()
		next := math.Float64bits(math.Float64frombits(cur) + v)

		if f.v.CompareAndSwap(cur, next) {
			return
		}
	}
}
