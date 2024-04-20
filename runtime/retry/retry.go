package retry

import (
	"context"
	"math"
	"time"

	"github.com/kanengo/akasar/runtime/urandom"
)

type Retry struct {
	options Options
	attempt int
}

type Options struct {
	BackOffMultiplier float64
	BackOfMinDuration time.Duration
}

var defaultOption = Options{
	BackOffMultiplier: 1.3,
	BackOfMinDuration: 10 * time.Millisecond,
}

func Begin() *Retry {
	return BeginWithOptions(defaultOption)
}

func BeginWithOptions(opts Options) *Retry {
	return &Retry{options: opts}
}

func (r *Retry) Continue(ctx context.Context) bool {
	if r.attempt != 0 {
		randomized(ctx, backOffDelay(r.attempt, r.options))
	}
	r.attempt++

	return ctx.Err() == nil
}

func (r *Retry) Reset() {
	r.attempt = 0
}

func backOffDelay(i int, opts Options) time.Duration {
	mult := math.Pow(opts.BackOffMultiplier, float64(i))
	return time.Duration(float64(opts.BackOfMinDuration) * mult)
}

func randomized(ctx context.Context, d time.Duration) {
	const jitter = 0.4
	mult := 1 - jitter*urandom.Float64() // 40%
	sleep(ctx, time.Duration(float64(d)*mult))
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-t.C:
	}
}
