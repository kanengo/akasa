package pool

import (
	"fmt"
	"testing"
)

func TestPowerOfTwoSizeBytes(t *testing.T) {
	bs, err := GetPowerOfTwoSizeBytes(100)
	if err != nil {
		t.Error(err)
		return
	}

	const s = "hello world, so beautiful"

	*bs = append(*bs, []byte(s)...)
	want := fmt.Sprintf("%p", *bs)
	if err := FreePowerOfTwoSizeBytes(*bs); err != nil {
		t.Error(err)
		return
	}

	bs, err = GetPowerOfTwoSizeBytes(111)
	if err != nil {
		t.Error(err)
		return
	}

	actual := fmt.Sprintf("%p", *bs)

	if want != actual {
		t.Error(fmt.Sprintf("want %s, actual: %s", want, actual))
		return
	}

	ts := make([]byte, 0, 4096)
	fmt.Println(len(ts))
	ts = ts[:500]
	fmt.Println(len(ts), ts[400])
}

func BenchmarkSliceLenCap(b *testing.B) {
	b.Run("t1", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			s := make([]byte, 0, 4098)
			_ = s
		}
	})
	b.Run("t2", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			s := make([]byte, 2500)
			_ = s
		}
	})
	b.Run("t3", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			s := make([]byte, 0, 4098)
			s = s[:2500]
			_ = s
		}
	})
}
