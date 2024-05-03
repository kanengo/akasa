package runtime

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	mu  sync.Mutex
	fns []func()
)

func OnExitSignal(f func()) {
	mu.Lock()
	defer mu.Unlock()
	fns = append(fns, f)
	if len(fns) != 1 {
		return
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sig
		mu.Lock()
		defer mu.Unlock()

		for i := len(fns) - 1; i >= 0; i-- {
			fns[i]()
		}

		if num, ok := s.(syscall.Signal); ok {
			os.Exit(128 + int(num))
		}
		os.Exit(1)
	}()

}
