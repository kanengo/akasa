package call

import (
	"log/slog"
	"time"

	"github.com/kanengo/akasar/runtime/logging"

	"go.opentelemetry.io/otel/trace"
)

const (
	defaultWritFlattenLimit      = 4 << 10
	defaultInlineHandlerDuration = 20 * time.Microsecond
)

type ClientOptions struct {
	Balancer
	Logger *slog.Logger

	// OptimisticSpinDuration 使用乐观锁自选等待结果的时间
	OptimisticSpinDuration time.Duration

	// 所有小于这个限制的写入数据,在都会被压成一个 buffer 再发送
	// 如果为0，会选一个合适的值， 如果是负值，这不生效
	WriteFlattenLimit int
}

func (c ClientOptions) withDefaults() ClientOptions {
	if c.Logger == nil {
		c.Logger = logging.StderrLogger(logging.Options{})
	}

	if c.Balancer == nil {
		c.Balancer = RoundRobin()
	}

	if c.WriteFlattenLimit == 0 {
		c.WriteFlattenLimit = defaultWritFlattenLimit
	}

	return c
}

type CallOptions struct {
	Retry bool

	ShardKey uint64
}

type ServerOptions struct {
	Logger *slog.Logger
	Tracer trace.Tracer

	InlineHandlerDuration time.Duration
	WriteFlattenLimit     int
}

func (s ServerOptions) withDefaults() ServerOptions {
	if s.Logger == nil {
		s.Logger = logging.StderrLogger(logging.Options{})
	}
	if s.Tracer == nil {

	}

	if s.InlineHandlerDuration == 0 {
		s.InlineHandlerDuration = defaultInlineHandlerDuration
	}

	if s.WriteFlattenLimit == 0 {
		s.WriteFlattenLimit = defaultWritFlattenLimit
	}

	return s
}
