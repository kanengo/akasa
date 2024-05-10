package akasar

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/kanengo/akasar/internal/reflection"

	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/akasar"

	"github.com/kanengo/akasar/runtime/codegen"

	"go.opentelemetry.io/otel/trace"
)

const HealthURL = "/debug/akasar/health"

var healthInit sync.Once

func Run[T any, P PointerToRoot[T]](ctx context.Context, app func(context.Context, *T) error) error {
	healthInit.Do(func() {
		http.HandleFunc(HealthURL, HealthHandler)
	})

	bootstrap, err := runtime.GetBootstrap(ctx)
	if err != nil {
		return err
	}

	if !bootstrap.Exists() {
		return runLocal[T, P](ctx, app)
	}

	return runRemote[T, P](ctx, app, bootstrap)
}

type PointerToRoot[T any] interface {
	*T
	InstanceOf[Root]
}

func runLocal[T any, _ PointerToRoot[T]](ctx context.Context, app func(context.Context, *T) error) error {
	opts := akasar.SingleAkasaletOptions{
		ConfigFilename: "",
		Config:         "",
		Quiet:          false,
		Fakes:          nil,
	}

	if filename := os.Getenv("SERVICEAKASAR_CONFIG"); filename != "" {
		contents, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("config file: %w", err)
		}
		opts.ConfigFilename = filename
		opts.Config = string(contents)
	}

	regs := codegen.Registered()
	if err := validateRegistrations(regs); err != nil {
		return err
	}

	alet, err := akasar.NewSingleAkasaLet(ctx, regs, opts)
	if err != nil {
		return err
	}

	go func() {
		if err := alet.ServeStatus(ctx); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
	}()

	root, err := alet.GetImpl(reflection.Type[T]())
	if err != nil {
		return err
	}

	return app(ctx, root.(*T))
}

func runRemote[T any, _ PointerToRoot[T]](ctx context.Context, app func(context.Context, *T) error, bootstrap runtime.Bootstrap) error {
	regs := codegen.Registered()
	if err := validateRegistrations(regs); err != nil {
		return err
	}
	opts := akasar.RemoteAkasaletOptions{
		Fakes:         nil,
		InjectRetries: 0,
	}
	alet, err := akasar.NewRemoteAkasaLet(ctx, regs, bootstrap, opts)
	if err != nil {
		return err
	}

	errs := make(chan error, 2)
	if alet.Args().RunMain {
		root, err := alet.GetImpl(reflection.Type[T]())
		if err != nil {
			return err
		}
		go func() {
			errs <- app(ctx, root.(*T))
		}()
	}
	go func() {
		errs <- alet.Wait()
	}()

	return <-errs
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

func (r *Result[T]) Unwrap() T {
	if r.err != nil {
		panic(codegen.WrapResultError(r.err))
	}
	return r.val
}

func ResultError[T any](errs ...error) Result[T] {
	var zero T
	return NewResult(zero, errs...)
}

var placeholder struct{}

func ResultUnwrap(errs ...error) {
	r := ResultError[placeholder](errs...)
	r.Unwrap()
}

func NewResult[T any](value T, errs ...error) Result[T] {
	var err error
	if len(errs) == 1 {
		err = errs[0]
	} else if len(errs) > 1 {
		err = errors.Join(errs...)
	}
	return Result[T]{
		val: value,
		err: err,
	}
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

func (c *Components[T]) Logger(ctx context.Context) *slog.Logger {
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

func (c *Components[T]) setLogger(logger *slog.Logger) {
	c.logger = logger
}

func (c *Components[T]) setAkasarInfo(info *akasar.Info) {
	c.akasarInfo = info
}

// components 实现InstanceOf接口
func (*Components[T]) components(T) {}

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
	proxyAddr string
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
