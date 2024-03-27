package akasar

import (
	"context"
	"log/slog"
	"net"

	"go.opentelemetry.io/otel/trace"
)

type Ref[T any] struct {
	val T
}

func (r *Ref[T]) Get() T {
	return r.val
}

func (r *Ref[T]) isRef() {}

func (r *Ref[T]) setRef(value any) {
	r.val = value.(T)
}

type Result[T any] struct {
	val T
	err error
}

type ResultUnwrapError error

func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(ResultUnwrapError(r.err))
	}
	return r.val
}

func (r *Result[T]) Error() error {
	return r.err
}

func (r *Result[T]) Value() T {
	return r.val
}

// InstanceOf 组件实例标识
type InstanceOf[T any] interface {
	components(T)
}

type Components[T any] struct {
	logger *slog.Logger
}

func (c Components[T]) Logger(ctx context.Context) *slog.Logger {
	logger := c.logger
	sc := trace.SpanContextFromContext(ctx)
	if sc.HasTraceID() {
		logger = logger.With(slog.String("traceId", sc.TraceID().String()))
	}
	if sc.HasSpanID() {
		logger = logger.With(slog.String("spanId", sc.TraceID().String()))
	}

	return logger
}

func (c Components[T]) setLogger(logger *slog.Logger) {
	c.logger = logger
}

// components 实现InstanceOf接口
func (Components[T]) components(T) {}

type Root struct {
}

type Initializer interface {
	Init(context.Context) error
}

type Listener struct {
	net.Listener
	Addr string
}

type WithRouter[T any] struct {
}

//=======================================

type Singleton[T any] struct {
}

func GetSingleton[T any]() {

}
