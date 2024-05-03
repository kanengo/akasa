package routing

import "testing"

func TestNextPowerOfTwo(t *testing.T) {
	for _, test := range []struct{ x, want int }{
		{0, 1}, {1, 1},
		{2, 2},
		{3, 4}, {4, 4},
		{5, 8}, {6, 8}, {7, 8}, {8, 8},
		{9, 16}, {10, 16}, {11, 16}, {12, 16}, {13, 16}, {14, 16}, {15, 16}, {16, 16},
		{1000, 1024},
	} {
		if got, want := nextPowerOfTwo(test.x), test.want; got != want {
			t.Errorf("nextPowerOfTwo(%d): got %d, want %d", test.x, got, want)
		}
	}
}
