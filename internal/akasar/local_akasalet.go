package akasar

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/kanengo/akasar/runtime/logging"

	"github.com/kanengo/akasar/internal/config"

	"github.com/kanengo/akasar/internal/env"
	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/traceio"
	"github.com/kanengo/akasar/runtime/protos"

	"github.com/kanengo/akasar/internal/tool/local"

	"github.com/kanengo/akasar/runtime/traces"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"go.opentelemetry.io/otel/trace"

	"github.com/kanengo/akasar/runtime/codegen"
)

type LocalAkasalet struct {
	ctx context.Context

	config *local.LocalConfig
	// Registration. 已注册的组件
	regs       []*codegen.Registration
	regsByName map[string]*codegen.Registration
	regsByIntf map[reflect.Type]*codegen.Registration
	regsByImpl map[reflect.Type]*codegen.Registration

	opts         LocalAkasaletOptions
	deploymentId string
	id           string
	createdAt    time.Time

	tracer trace.Tracer
	mu     sync.Mutex

	components map[string]any

	quiet bool

	info *Info
}

type LocalAkasaletOptions struct {
	ConfigFilename string // config filename
	Config         string //config contents
	Quiet          bool
	Fakes          map[reflect.Type]any
}

func parseLocalConfig(reg []*codegen.Registration, filename, contents string) (*local.LocalConfig, error) {
	config := &local.LocalConfig{
		App: &protos.AppConfig{},
	}

	if contents != "" {
		app, err := runtime.ParseConfig(filename, contents, codegen.ComponentConfigValidator)
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		config.App = app
	}

	if config.App.Name == "" {
		config.App.Name = filepath.Base(os.Args[0])
	}
	listeners := map[string]struct{}{}
	for _, reg := range reg {
		for _, listener := range reg.Listeners {
			listeners[listener] = struct{}{}
		}
	}

	for listener := range config.Listeners {
		if _, ok := listeners[listener]; !ok {
			return nil, fmt.Errorf("listener %s not found", listener)
		}
	}

	return config, nil
}

func NewLocalAkasaLet(ctx context.Context, regs []*codegen.Registration, opts LocalAkasaletOptions) (*LocalAkasalet, error) {
	// Parse localConfig.
	localConfig, err := parseLocalConfig(regs, opts.ConfigFilename, opts.Config)
	if err != nil {
		return nil, err
	}

	envKvs, err := env.Parse(localConfig.App.Env)
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

	tracer, err := localTracer(ctx, "", deploymentId, id)
	if err != nil {
		return nil, err
	}

	return &LocalAkasalet{
		ctx:          ctx,
		config:       localConfig,
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
	}, nil
}

func (l *LocalAkasalet) GetIface(t reflect.Type) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.getIntf(t, "root")
}

func (l *LocalAkasalet) GetImpl(t reflect.Type) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.getImpl(t)
}

func (l *LocalAkasalet) getIntf(t reflect.Type, requester string) (any, error) {
	reg, ok := l.regsByIntf[t]
	if !ok {
		return nil, fmt.Errorf("component %v not found; maybe you forgot to run akasar generate", t)
	}
	c, err := l.get(reg)
	if err != nil {
		return nil, err
	}

	return reg.LocalStubFn(c, requester, l.tracer), nil
}

func (l *LocalAkasalet) getImpl(t reflect.Type) (any, error) {
	reg, ok := l.regsByImpl[t]
	if !ok {
		return nil, fmt.Errorf("component %v not found; maybe you forgot to run akasar generate", t)
	}

	return l.get(reg)
}

type localLogWriter struct {
	quiet bool
}

func (w localLogWriter) Write(p []byte) (n int, err error) {
	if w.quiet {
		return
	}
	return os.Stderr.Write(p)
}

func (l *LocalAkasalet) logger(name string) *slog.Logger {
	logger := slog.New(logging.NewLogHandler(localLogWriter{quiet: l.quiet}, logging.Options{
		App:        l.config.App.Name,
		Deployment: l.deploymentId,
		Component:  name,
		Attrs:      nil,
	}, slog.LevelDebug))

	return logger
}

func (l *LocalAkasalet) get(reg *codegen.Registration) (any, error) {
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

	// Set logger
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

func localTracer(ctx context.Context, app, deploymentId, id string) (trace.Tracer, error) {
	traceDB, err := traces.OpenDB(ctx, local.TracesDBFile)
	if err != nil {
		return nil, fmt.Errorf("cannot open traces db: %w", err)
	}
	exporter := traceio.NewWriter(func(spans *protos.TraceSpans) error {
		return traceDB.Store(ctx, app, deploymentId, spans)
	})

	return tracer(exporter, app, deploymentId, id), nil
}
