package pool

import (
	"fmt"
	"github.com/kanengo/akasar/internal/umath"
	"math"
	"sync"
)

var powerOfTwoSizeBytesPools map[int]*sync.Pool

func init() {
	powerOfTwoSizeBytesPools = make(map[int]*sync.Pool, 33)

	for n := 1; n <= math.MaxUint32+1; n *= 2 {
		powerOfTwoSizeBytesPools[n] = &sync.Pool{New: func() any {
			s := make([]byte, 0, n)
			return &s
		}}
	}
}

func GetPowerOfTwoSizeBytes(size int) (*[]byte, error) {
	if size == 0 {
		return nil, nil
	}
	size = umath.NextPowerOfTwo(size)
	p, ok := powerOfTwoSizeBytesPools[size]
	if !ok {
		return nil, fmt.Errorf("get size %d does not match the pool", size)
	}

	bs := p.Get().(*[]byte)
	//slog.Info("[PowerOfTwoSizeBytes]get", "size", size, "addr", fmt.Sprintf("%p", *bs), "stack", stack)
	return bs, nil
}

func FreePowerOfTwoSizeBytes(data []byte) error {
	if data == nil {
		return nil
	}
	size := cap(data)
	if size == 0 {
		return nil
	}
	p, ok := powerOfTwoSizeBytesPools[size]
	if !ok {
		return fmt.Errorf("free size %d does not match the pool", size)
	}
	data = data[:0]
	p.Put(&data)
	//slog.Info("[PowerOfTwoSizeBytes]free", "size", size, "addr", fmt.Sprintf("%p", data))
	return nil
}
