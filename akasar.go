package akasar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/kanengo/akasar/internal/akasar"

	"github.com/kanengo/akasar/runtime/codegen"

	"go.opentelemetry.io/otel/trace"
)

func Run[T any, P PointerToRoot[T]](ctx context.Context, app func(context.Context, *T) error) error {
	return nil
}

type PointerToRoot[T any] interface {
	*T
	InstanceOf[Root]
}

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

type ResultUnwrapError struct {
	Err error
}

func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(ResultUnwrapError{r.err})
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
	logger     *slog.Logger
	akasarInfo *akasar.Info
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

func (c Components[T]) setAkasarInfo(info *akasar.Info) {
	c.akasarInfo = info
}

// components 实现InstanceOf接口
func (Components[T]) components(T) {}

type Root interface {
}

type WithConfig[T any] struct {
	config T
}

func (wc *WithConfig[T]) Config() *T {
	return &wc.config
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

type NotRetriable interface {
}

var RemoteCallError = errors.New("service akasar remote call error")

var HealthHandler = func(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "OK")
}

type AutoMarshal struct {
}

func (*AutoMarshal) AkasarMarshal(enc *codegen.Serializer)     {}
func (*AutoMarshal) AkasarUnmarshal(enc *codegen.Deserializer) {}

//=======================================

type Singleton[T any] struct {
}

func GetSingleton[T any]() {

}
