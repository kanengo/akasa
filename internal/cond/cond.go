package cond

import (
	"context"
	"sync"
)

// Cond is a context-aware version of a sync.Cond. Like a sync.Cond, a Cond
// must not be copied after first use.
type Cond struct {
	L sync.Locker

	mu      sync.Mutex
	waiters []chan struct{}
}

func NewCond(l sync.Locker) *Cond {
	return &Cond{L: l}
}

func (c *Cond) Broadcast() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, waiter := range c.waiters {
		close(waiter)
	}

	c.waiters = nil
}

func (c *Cond) Signal() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.waiters) == 0 {
		return
	}
	waiter := c.waiters[0]
	c.waiters = c.waiters[1:]
	close(waiter)
}

func (c *Cond) Wait(ctx context.Context) error {
	waiter := make(chan struct{})
	c.mu.Lock()
	c.waiters = append(c.waiters, waiter)
	c.mu.Unlock()

	c.L.Unlock()
	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-waiter:
	}
	c.L.Lock()
	return err
}
