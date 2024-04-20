package urandom

import (
	"math/rand"
	"sync"
	"time"
)

var pool = sync.Pool{
	New: func() any {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		return rng
	},
}

func Float64() float64 {
	r := pool.Get().(*rand.Rand)
	defer pool.Put(r)

	return r.Float64()
}

func Int63() int64 {
	r := pool.Get().(*rand.Rand)
	defer pool.Put(r)

	return r.Int63()
}
