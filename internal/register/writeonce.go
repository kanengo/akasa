package register

import (
	"fmt"
	"sync"
)

// WriteOnce 保证对应的val只会被写入一次，读取值时若未被写入，则等待直至值被其他goroutine写入
type WriteOnce[T any] struct {
	mu      sync.Mutex
	c       sync.Cond
	written bool
	val     T
}

func (w *WriteOnce[T]) Write(val T) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()

	if w.written {
		panic(fmt.Sprintf("WriteOnce written more than once %v, new %v", w.val, val))
	}

	w.val = val
	w.written = true
	w.c.Broadcast()

}

func (w *WriteOnce[T]) TryWrite(val T) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()

	if w.written {
		return false
	}
	w.val = val
	w.written = true
	w.c.Broadcast()
	return true
}

func (w *WriteOnce[T]) Read() T {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.init()

	for !w.written {
		w.c.Wait()
	}

	return w.val
}

func (w *WriteOnce[T]) init() {
	if w.c.L == nil {
		w.c.L = &w.mu
	}
}
