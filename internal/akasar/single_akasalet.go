package akasar

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"
	"time"

	"github.com/kanengo/akasar/runtime/logging"

	"github.com/kanengo/akasar/internal/config"

	"github.com/kanengo/akasar/internal/env"
	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/traceio"
	"github.com/kanengo/akasar/runtime/protos"

	"github.com/kanengo/akasar/internal/tool/single"

	"github.com/kanengo/akasar/runtime/traces"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"go.opentelemetry.io/otel/trace"

	"github.com/kanengo/akasar/runtime/codegen"
)

type SingleAkasalet struct {
	ctx context.Context

	config *single.SingleConfig
	// Registration. 已注册的组件
	regs       []*codegen.Registration
	regsByName map[string]*codegen.Registration
	regsByIntf map[reflect.Type]*codegen.Registration
	regsByImpl map[reflect.Type]*codegen.Registration

	opts         SingleAkasaletOptions
	deploymentId string
	id           string
	createdAt    time.Time

	tracer trace.Tracer
	mu     sync.Mutex

	components map[string]any

	quiet bool

	info *Info

	listeners map[string]net.Listener
}

type SingleAkasaletOptions struct {
	ConfigFilename string // config filename
	Config         string //config contents
	Quiet          bool
	Fakes          map[reflect.Type]any
}

func parseSingleConfig(reg []*codegen.Registration, filename, contents string) (*single.SingleConfig, error) {
	singleConfig := &single.SingleConfig{
		App: &protos.AppConfig{},
	}

	if contents != "" {
		app, err := runtime.ParseConfig(filename, contents, codegen.ComponentConfigValidator)
		if err != nil {
			return nil, fmt.Errorf("parse singleConfig: %w", err)
		}
		if err := runtime.ParseConfigSection(single.ConfigKey, single.ConfigShortKey, app.Sections, singleConfig); err != nil {
			return nil, fmt.Errorf("parse singleConfig: %w", err)
		}
		singleConfig.App = app
	}

	if singleConfig.App.Name == "" {
		singleConfig.App.Name = filepath.Base(os.Args[0])
	}

	listeners := map[string]struct{}{}
	for _, reg := range reg {
		for _, listener := range reg.Listeners {
			listeners[listener] = struct{}{}
		}
	}

	for listener := range singleConfig.Listeners {
		if _, ok := listeners[listener]; !ok {
			return nil, fmt.Errorf("listener %s not found", listener)
		}
	}

	return singleConfig, nil
}

func NewSingleAkasaLet(ctx context.Context, regs []*codegen.Registration, opts SingleAkasaletOptions) (*SingleAkasalet, error) {
	// Parse singleConfig.
	singleConfig, err := parseSingleConfig(regs, opts.ConfigFilename, opts.Config)
	if err != nil {
		return nil, err
	}

	envKvs, err := env.Parse(singleConfig.App.Env)
	if err != nil {
		return nil, err
	}
	for k, v := range envKvs {
		if err := os.Setenv(k, v); err != nil {
			return nil, err
		}
	}

	deploymentId := gonanoid.Must(16)
	id := gonanoid.Must(16)

	regsByName := map[string]*codegen.Registration{}
	regsByIntf := map[reflect.Type]*codegen.Registration{}
	regsByImpl := map[reflect.Type]*codegen.Registration{}
	for _, reg := range regs {
		regsByName[reg.Name] = reg
		regsByIntf[reg.Iface] = reg
		regsByImpl[reg.Impl] = reg
	}

	tracer, err := singleTracer(ctx, "", deploymentId, id)
	if err != nil {
		return nil, err
	}

	return &SingleAkasalet{
		ctx:          ctx,
		config:       singleConfig,
		regs:         regs,
		regsByName:   regsByName,
		regsByIntf:   regsByIntf,
		regsByImpl:   regsByImpl,
		opts:         opts,
		deploymentId: deploymentId,
		id:           id,
		createdAt:    time.Now(),
		tracer:       tracer,
		info:         &Info{DeploymentID: deploymentId},
		components:   make(map[string]any),
		listeners:    make(map[string]net.Listener),
	}, nil
}

func (l *SingleAkasalet) GetIface(t reflect.Type) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.getIntf(t, "root")
}

func (l *SingleAkasalet) GetImpl(t reflect.Type) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.getImpl(t)
}

func (l *SingleAkasalet) getIntf(t reflect.Type, requester string) (any, error) {
	reg, ok := l.regsByIntf[t]
	if !ok {
		return nil, fmt.Errorf("getIntf: component %v not found; maybe you forgot to run akasar generate", t)
	}
	c, err := l.get(reg)
	if err != nil {
		return nil, err
	}

	return reg.LocalStubFn(c, requester, l.tracer), nil
}

func (l *SingleAkasalet) getImpl(t reflect.Type) (any, error) {
	reg, ok := l.regsByImpl[t]
	if !ok {
		return nil, fmt.Errorf("getImpl: component %v not found; maybe you forgot to run akasar generate", t)
	}

	return l.get(reg)
}

type singleLogWriter struct {
	quiet bool
}

func (w singleLogWriter) Write(p []byte) (n int, err error) {
	if w.quiet {
		return
	}
	return os.Stdout.Write(p)
}

func (l *SingleAkasalet) logger(name string) *slog.Logger {
	pp := logging.NewPrettyPrinter()
	logger := slog.New(logging.NewLogHandler(func(entry *protos.LogEntry) {
		if l.quiet {
			return
		}
		if entry.Level == slog.LevelError.String() {
			_, _ = fmt.Fprintln(os.Stderr, pp.Format(entry))
		} else {
			_, _ = fmt.Fprintln(os.Stdout, pp.Format(entry))
		}

	}, logging.Options{
		App:        l.config.App.Name,
		Deployment: l.deploymentId,
		Component:  name,
		Akasalet:   l.id,
		Level:      l.config.App.LogLevel,
	}))

	return logger
}

func (l *SingleAkasalet) get(reg *codegen.Registration) (any, error) {
	if c, ok := l.components[reg.Name]; ok {
		return c, nil
	}

	if fake, ok := l.opts.Fakes[reg.Iface]; ok {
		return fake, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// Fill config
	if cfg := config.ComponentConfig(v); cfg != nil {
		if err := runtime.ParseConfigSection(reg.Name, "", l.config.App.Sections, cfg); err != nil {
			return nil, err
		}
	}

	// Set sysLogger
	if err := SetLogger(obj, l.logger(reg.Name)); err != nil {
		return nil, err
	}

	// Set info
	if err := SetAkasarInfo(obj, l.info); err != nil {
		return nil, err
	}

	// Fill ref fields.
	if err := FillRefs(obj, func(t reflect.Type) (any, error) {
		return l.getIntf(t, reg.Name)
	}); err != nil {
		return nil, err
	}

	// Fill listener fields.
	if err := FillListeners(obj, func(name string) (net.Listener, string, error) {
		lis, err := l.listen(name)
		if err != nil {
			return nil, "", err
		}

		return lis, "", nil
	}); err != nil {
		return nil, err
	}

	//Call Init if available.
	if i, ok := obj.(interface {
		Init(ctx context.Context) error
	}); ok {
		if err := i.Init(l.ctx); err != nil {
			return nil, fmt.Errorf("component %q initalization failed: %w", reg.Name, err)
		}
	}

	l.components[reg.Name] = obj

	return obj, nil
}

func (l *SingleAkasalet) listen(name string) (net.Listener, error) {
	if lis, ok := l.listeners[name]; ok {
		return lis, nil
	}

	var addr string
	if opts, ok := l.config.Listeners[name]; ok {
		addr = opts.Address
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	l.listeners[name] = lis

	return lis, nil
}

func (l *SingleAkasalet) ServeStatus(ctx context.Context) error {
	noopLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError + 1,
	}))

	_ = noopLogger

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	mux := http.NewServeMux()
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}
	errs := make(chan error, 1)
	go func() {
		errs <- serveHTTP(ctx, lis, mux)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		code := 0
		os.Exit(code)
	}

	return nil
}

func singleTracer(ctx context.Context, app, deploymentId, id string) (trace.Tracer, error) {
	traceDB, err := traces.OpenDB(ctx, single.TracesDBFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open traces db: %w", err)
	}
	exporter := traceio.NewWriter(func(spans *protos.TraceSpans) error {
		return traceDB.Store(ctx, app, deploymentId, spans)
	})

	return tracer(exporter, app, deploymentId, id), nil
}

func serveHTTP(ctx context.Context, lis net.Listener, handler http.Handler) error {
	svr := http.Server{Handler: handler}
	errs := make(chan error, 1)
	go func() {
		errs <- svr.Serve(lis)
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return svr.Shutdown(ctx)
	}
}
