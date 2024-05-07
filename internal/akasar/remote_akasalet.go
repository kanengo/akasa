package akasar

import (
	"context"
	"fmt"
	"github.com/kanengo/akasar/internal/config"
	"golang.org/x/exp/maps"
	"log/slog"
	"net"
	"os"
	"reflect"
	"sync"

	"github.com/kanengo/akasar/runtime/version"

	"github.com/kanengo/akasar/runtime/deployers"

	"github.com/kanengo/akasar/runtime/retry"

	"github.com/kanengo/akasar/runtime/logging"

	"github.com/kanengo/akasar/internal/net/call"

	"github.com/kanengo/akasar/internal/register"

	"github.com/kanengo/akasar/internal/traceio"

	"go.opentelemetry.io/otel/trace"

	"github.com/kanengo/akasar/internal/errgroup"

	"github.com/kanengo/akasar/runtime"
	"github.com/kanengo/akasar/runtime/codegen"

	"github.com/kanengo/akasar/runtime/protos"

	"github.com/kanengo/akasar/internal/control"
)

type RemoteAkasaLet struct {
	ctx       context.Context
	servers   *errgroup.Group
	opts      RemoteAkasaletOptions
	args      *protos.AkasaletArgs
	dialAddr  string
	id        string
	info      *Info
	deployer  control.DeployerControl
	tracer    trace.Tracer
	logDst    *remoteLogger
	sysLogger *slog.Logger

	// 与 Envelope 发起的初始化握手同步的状态
	initMu     sync.Mutex
	initCalled bool
	initDone   chan struct{}

	// 当 initDone 关闭后可使用
	sectionConfig map[string]string

	deployerReady chan struct{}

	componentsByName map[string]*component
	componentsByIntf map[reflect.Type]*component
	componentsByImpl map[reflect.Type]*component
	redirects        map[string]redirect

	lisMu     sync.Mutex
	listeners map[string]*listener
}

type component struct {
	reg *codegen.Registration

	activateInit sync.Once // 激活组件
	activateErr  error

	implInit   sync.Once // 初始化 impl, serverStub
	implErr    error
	impl       any            //组件接口的实例
	serverStub codegen.Server // 处理其他进程的远程调用

	resolver *routingResolver
	balance  *routingBalancer

	stubInit sync.Once // 初始化 stub
	stubErr  error
	stub     codegen.Stub

	local register.WriteOnce[bool]

	//TODO(leeka)
	// load
}

type redirect struct {
	component *component
	target    string
	address   string
}

type listener struct {
	lis       net.Listener
	proxyAddr string
}

type RemoteAkasaletOptions struct {
	Fakes         map[reflect.Type]any
	InjectRetries int
}

func NewRemoteAkasaLet(ctx context.Context, regs []*codegen.Registration, bootstrap runtime.Bootstrap, opts RemoteAkasaletOptions) (*RemoteAkasaLet, error) {
	args := bootstrap.Args
	if args == nil {
		return nil, fmt.Errorf("missing akasalet arguments")
	}

	if err := runtime.CheckAkasaletArgs(args); err != nil {
		return nil, err
	}

	// Make internal listener, use to rpc
	// TODO(leeka) support quic
	netPoint, err := call.ParseNetEndpoint(args.InternalAddress)
	if err != nil {
		return nil, err

	}
	lis, err := net.Listen(netPoint.Net, netPoint.Addr)
	if err != nil {
		return nil, err
	}
	cleanupListener := true
	defer func() {
		if cleanupListener {
			_ = lis.Close()
		}
	}()

	dialAddr := fmt.Sprintf("%s://%s", lis.Addr().Network(), lis.Addr().String())

	servers, ctx := errgroup.WithContext(ctx)

	a := &RemoteAkasaLet{
		ctx:              ctx,
		servers:          servers,
		opts:             opts,
		args:             args,
		dialAddr:         dialAddr,
		id:               args.Id,
		info:             &Info{DeploymentID: args.DeploymentId},
		logDst:           newRemoteLogger(os.Stderr),
		initDone:         make(chan struct{}),
		deployerReady:    make(chan struct{}),
		componentsByName: make(map[string]*component),
		componentsByIntf: make(map[reflect.Type]*component),
		componentsByImpl: make(map[reflect.Type]*component),
		redirects:        make(map[string]redirect),
		listeners:        make(map[string]*listener),
	}

	// communicate with deployer
	controlSocket, err := net.Listen("unix", args.ControlSocket)
	if err != nil {
		return nil, err
	}

	a.sysLogger = a.getLogger("akasalet")

	traceExporter := traceio.NewWriter(a.exportTraceSpans)
	a.tracer = tracer(traceExporter, args.App, args.DeploymentId, args.Id)

	// 初始化component
	for _, reg := range regs {
		c := &component{
			reg: reg,
		}
		a.componentsByName[reg.Name] = c
		a.componentsByIntf[reg.Iface] = c
		a.componentsByImpl[reg.Impl] = c

		if reg.Routed {
			//TODO(leeka) support routed component
		}

		c.resolver = newRoutingResolver()
		c.balance = newRoutingBalancer()
	}

	for _, r := range args.Redirects {
		c, ok := a.componentsByName[r.Component]
		if !ok {
			return nil, fmt.Errorf("redirect names unknown component: %q", r.Component)
		}
		a.redirects[r.Component] = redirect{
			component: c,
			target:    r.Target,
			address:   r.Address,
		}
	}

	a.deployer, err = a.getDeployerControl()
	if err != nil {
		return nil, err
	}
	close(a.deployerReady)

	// Serve the akasalet control component.
	servers.Go(func() error {
		return deployers.ServeComponents(ctx, controlSocket, a.sysLogger, map[string]any{
			control.AkasaletPath: a,
		})
	})

	// 等待初始化握手完成.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-a.initDone:
		// ready to serve
	}

	// log writing.
	servers.Go(func() error {
		a.logDst.run(ctx, a.deployer.LogBatch)
		return nil
	})

	cleanupListener = false //
	// Serve RPC request from other akasalet
	servers.Go(func() error {
		svr := &server{Listener: lis, alet: a}
		opts := call.ServerOptions{
			Logger: a.sysLogger,
			Tracer: a.tracer,
		}
		if err := call.Serve(a.ctx, svr, opts); err != nil {
			a.sysLogger.Error("RPC server error", "err", err)
			return err
		}
		return nil
	})

	a.sysLogger.Debug("akasalet started", "addr", dialAddr)
	return a, nil

}

func (a *RemoteAkasaLet) Args() *protos.AkasaletArgs {
	return a.args
}

func (a *RemoteAkasaLet) exportTraceSpans(spans *protos.TraceSpans) error {
	select {
	case <-a.deployerReady:
		return a.deployer.HandlerTraceSpans(context.TODO(), spans)
	default:
		return fmt.Errorf("dropping trace spans since deployer is not avaliable yet")
	}
}

func (a *RemoteAkasaLet) InitAkasalet(ctx context.Context, req *protos.InitAkasaletRequest) (*protos.InitAkasaletReply, error) {
	a.initMu.Lock()
	defer a.initMu.Unlock()
	if !a.initCalled {
		a.sectionConfig = req.Sections
		a.initCalled = true
		close(a.initDone)
	}

	return &protos.InitAkasaletReply{
		DialAddr: a.dialAddr,
		Version: &protos.SemVer{
			Major: version.DeployerMajor,
			Minor: version.DeployerMinor,
			Patch: 0,
		},
	}, nil
}

func (a *RemoteAkasaLet) UpdateComponents(ctx context.Context, req *protos.UpdateComponentsRequest) (*protos.UpdateComponentsReply, error) {
	var errs []error
	var components []*component
	var shortened []string
	// 过滤掉不存在的 component
	for _, com := range req.Components {
		short := logging.ShortenComponent(com)
		shortened = append(shortened, short)
		c, err := a.getComponent(com)
		if err != nil {
			a.sysLogger.Error("Failed to update", "component", short, "err", err)
			errs = append(errs, err)
			continue
		}
		components = append(components, c)
	}

	for i, c := range components {
		go func() {
			a.sysLogger.Debug("Updating", "component", shortened[i])
			if _, err := a.GetImpl(c.reg.Impl); err != nil {
				a.sysLogger.Error("Failed to update", "component", shortened[i], "err", err)
				return
			}
			a.sysLogger.Debug("Updated", "component", shortened[i])
		}()
	}

	return &protos.UpdateComponentsReply{}, nil
}

func (a *RemoteAkasaLet) UpdateRoutingInfo(ctx context.Context, req *protos.UpdateRoutingInfoRequest) (reply *protos.UpdateRoutingInfoReply, err error) {
	if req.RoutingInfo == nil {
		a.sysLogger.Error("Failed to update routing info")
		return nil, fmt.Errorf("nil routing info")
	}

	info := req.RoutingInfo
	defer func() {
		name := logging.ShortenComponent(info.Component)
		routing := fmt.Sprint(info.Replicas)
		if info.Local {
			routing = "local"
		}
		if err != nil {
			a.sysLogger.Error("Failed to update routing info", "component", name, "addr", routing, "err", err)
		} else {
			a.sysLogger.Debug("Updated routing info", "component", name, "addr", routing)
		}
	}()

	c, err := a.getComponent(info.Component)
	if err != nil {
		return nil, err
	}

	c.local.TryWrite(info.Local)
	if got, want := c.local.Read(), info.Local; got != want {
		return nil, fmt.Errorf("RoutingInfo.Local for %q: got %t, want %t", info.Component, got, want)
	}

	if info.Local {
		if len(info.Replicas) > 0 {
			a.sysLogger.Error("Local routing has replicas", "component", info.Component, "replicas", info.Replicas)
		}

		if info.Assignment != nil {
			a.sysLogger.Error("Local routing has assignment", "component", info.Component, "assignment", info.Assignment)
		}
		return
	}

	endpoints, err := parseEndpoints(info.Replicas)
	if err != nil {
		return nil, err
	}
	c.resolver.update(endpoints)

	if info.Assignment != nil {
		c.balance.update(info.Assignment)
	}

	return &protos.UpdateRoutingInfoReply{}, nil
}

func (a *RemoteAkasaLet) GetHealth(ctx context.Context, request *protos.GetHealthRequest) (*protos.GetHealthReply, error) {
	return &protos.GetHealthReply{Status: protos.HealthStatus_HEALTHY}, nil
}

func (a *RemoteAkasaLet) GetLoad(ctx context.Context, request *protos.GetLoadRequest) (*protos.GetLoadReply, error) {
	//TODO implement me
	panic("implement me")
}

func (a *RemoteAkasaLet) GetMetrics(ctx context.Context, request *protos.GetMetricsRequest) (*protos.GetMetricsReply, error) {
	//TODO implement me
	panic("implement me")
}

func (a *RemoteAkasaLet) GetProfile(ctx context.Context, request *protos.GetProfileRequest) (*protos.GetProfileReply, error) {
	//TODO implement me
	panic("implement me")
}

func (a *RemoteAkasaLet) getDeployerControl() (control.DeployerControl, error) {
	r, ok := a.redirects[control.DeployerPath]
	if !ok {
		return nil, fmt.Errorf("rediected deployer control component not found")
	}
	com, err := a.getIntf(r.component.reg.Iface, r.target)
	if err != nil {
		return nil, err
	}

	deployer, ok := com.(control.DeployerControl)
	if !ok {
		return nil, fmt.Errorf("rediected component of type %T does not implement DeployerControl", com)
	}

	return deployer, nil
}

// getIntf
func (a *RemoteAkasaLet) getIntf(t reflect.Type, requester string) (any, error) {
	c, ok := a.componentsByIntf[t]
	if !ok {
		return nil, fmt.Errorf("component of type %v was not registered; maybe you forgot to run akasar generate", t)
	}

	// 重定向组件
	if r, ok := a.redirects[c.reg.Name]; ok {
		return a.redirect(requester, c, r.target, r.address)
	}

	// 激活组件, 一次激活就够
	c.activateInit.Do(func() {
		name := logging.ShortenComponent(c.reg.Name)
		a.sysLogger.Debug("Activating init component", "component", name)
		errMsg := fmt.Sprintf("cannot active component %q", c.reg.Name)

		c.activateErr = a.repeatedly(a.ctx, errMsg, func() error {
			request := &protos.ActivateComponentRequest{
				Component: c.reg.Name,
				Routed:    c.reg.Routed,
			}
			_, err := a.deployer.ActivateComponent(a.ctx, request)
			return err
		})
		if c.activateErr != nil {
			a.sysLogger.Error("Failed to activate", "component", name, "err", c.activateErr)
		} else {
			a.sysLogger.Debug("Activated", "component", name)
		}
	})
	if c.activateErr != nil {
		return nil, c.activateErr
	}

	// 本地组件
	if c.local.Read() {
		impl, err := a.GetImpl(c.reg.Impl)
		if err != nil {
			return nil, err
		}
		return c.reg.LocalStubFn(impl, requester, a.tracer), nil
	}

	stub, err := a.getStub(c)
	if err != nil {
		return nil, err
	}

	return c.reg.ClientStubFn(stub, requester, a.tracer), nil
}

// repeatedly 重复执行 f 直到成功或者 ctx 被取消
func (a *RemoteAkasaLet) repeatedly(ctx context.Context, errMsg string, f func() error) error {
	for r := retry.Begin(); r.Continue(ctx); {
		if err := f(); err != nil {
			a.sysLogger.Error(errMsg+"; will retry", "err", err)
			continue
		}
		return nil
	}

	return fmt.Errorf("%s: %w", errMsg, ctx.Err())
}

// redirect 为 c 创建一个组件接口，重定向到指定的地址
func (a *RemoteAkasaLet) redirect(requester string, c *component, target, address string) (any, error) {
	// 激活组件，因为是重定向，不需要做什么初始化
	c.activateInit.Do(func() {})

	// Make a special stub.
	c.stubInit.Do(func() {
		endpoint, err := call.ParseNetEndpoint(address)
		if err != nil {
			c.stubErr = err
			return
		}
		resolver := call.NewConstantResolver(endpoint)
		c.stub, c.stubErr = a.makeStub(target, c.reg, resolver, nil, false)
	})

	if c.stubErr != nil {
		return nil, c.stubErr
	}

	return c.reg.ClientStubFn(c.stub, requester, a.tracer), nil
}

func (a *RemoteAkasaLet) getStub(c *component) (codegen.Stub, error) {
	a.sysLogger.Debug("getStub before", "component", c.reg.Name)
	c.stubInit.Do(func() {
		c.stub, c.stubErr = a.makeStub(c.reg.Name, c.reg, c.resolver, c.balance, true)
	})
	a.sysLogger.Debug("getStub after", "component", c.reg.Name)
	return c.stub, c.stubErr
}

func (a *RemoteAkasaLet) makeStub(fullName string, reg *codegen.Registration, resolver call.Resolver, balancer call.Balancer, wait bool) (codegen.Stub, error) {
	name := logging.ShortenComponent(fullName)
	a.sysLogger.Debug("makeStub", "component", name)
	opts := call.ClientOptions{
		Balancer: balancer,
		Logger:   a.sysLogger,
	}
	conn, err := call.Connect(a.ctx, resolver, opts)
	if err != nil {
		a.sysLogger.Error("Failed to connect to remote", "component", name, "err", err)
		return nil, err
	}
	return call.NewStub(fullName, reg, conn, a.opts.InjectRetries), nil
}

func (a *RemoteAkasaLet) GetImpl(t reflect.Type) (any, error) {
	c, ok := a.componentsByImpl[t]
	if !ok {
		return nil, fmt.Errorf("component implementation of type %v was not registered; maybe you forgot to run akasar generate", t)
	}

	c.implInit.Do(func() {
		name := logging.ShortenComponent(c.reg.Name)
		a.sysLogger.Debug("Initializing component", "component", name)
		c.impl, c.implErr = a.createComponent(a.ctx, c.reg)
		if c.implErr != nil {
			a.sysLogger.Error("Failed to initialize component", "component", name, "err", c.implErr)
			return
		} else {
			a.sysLogger.Debug("Initialized component", "component", name)
		}
		c.serverStub = c.reg.ServerStubFn(c.impl)
	})

	return c.impl, c.implErr
}

func (a *RemoteAkasaLet) getLogger(fullName string, attrs ...slog.Attr) *slog.Logger {
	logOptions := logging.Options{
		App:        a.args.App,
		Deployment: a.args.DeploymentId,
		Component:  fullName,
		Akasalet:   a.id,
		Level:      a.args.LogLevel,
	}
	logger := slog.New(logging.NewLogHandler(a.logDst.log, logOptions))

	return logger
}

func (a *RemoteAkasaLet) createComponent(ctx context.Context, reg *codegen.Registration) (any, error) {
	if obj, ok := a.opts.Fakes[reg.Iface]; ok {
		return obj, nil
	}

	v := reflect.New(reg.Impl)
	obj := v.Interface()

	// Fill config
	if cfg := config.ComponentConfig(v); cfg != nil {
		if err := runtime.ParseConfigSection(reg.Name, "", a.sectionConfig, cfg); err != nil {
			return nil, err
		}
	}

	// Set sysLogger.
	if err := SetLogger(obj, a.getLogger(reg.Name)); err != nil {
		return nil, err
	}

	//
	if err := SetAkasarInfo(obj, a.info); err != nil {
		return nil, err
	}

	// Fill ref fields.
	if err := FillRefs(obj, func(t reflect.Type) (any, error) {
		return a.getIntf(t, reg.Name)
	}); err != nil {
		return nil, err
	}

	if err := FillListeners(obj, func(name string) (net.Listener, string, error) {
		lis, err := a.listener(ctx, name)
		if err != nil {
			return nil, "", err
		}
		return lis.lis, lis.proxyAddr, nil
	}); err != nil {
		return nil, err
	}

	if i, ok := obj.(interface{ Init(context.Context) error }); ok {
		if err := i.Init(ctx); err != nil {
			return nil, fmt.Errorf("component %q initialization failed: %w", reg.Name, err)
		}
	}

	return obj, nil
}

func (a *RemoteAkasaLet) listener(ctx context.Context, name string) (*listener, error) {
	a.lisMu.Lock()
	defer a.lisMu.Unlock()
	if lis, ok := a.listeners[name]; ok {
		return lis, nil
	}

	if name == "" {
		return nil, fmt.Errorf("empty listener for component %q", name)
	}

	// Get the address to listen on from deployer.
	addr, err := a.getListenerAddress(ctx, name)
	if err != nil {
		return nil, err
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %q: %w", addr, err)
	}

	// Export the listener to deployer.
	errMsg := fmt.Sprintf("listener(%q): error exporting listener %v", name, lis.Addr())
	var reply *protos.ExportListenerReply

	if err := a.repeatedly(a.ctx, errMsg, func() error {
		var err error
		request := &protos.ExportListenerRequest{
			Listener: name,
			Address:  lis.Addr().String(),
		}
		reply, err = a.deployer.ExportListener(ctx, request)
		return err
	}); err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, fmt.Errorf("listener(%q): %s", name, reply.Error)
	}

	l := &listener{
		lis:       lis,
		proxyAddr: reply.ProxyAddress,
	}
	a.listeners[name] = l
	return l, nil
}

func (a *RemoteAkasaLet) getListenerAddress(ctx context.Context, name string) (string, error) {
	request := &protos.GetListenerAddressRequest{Name: name}
	reply, err := a.deployer.GetListenerAddress(ctx, request)
	if err != nil {
		return "", err
	}

	return reply.Address, nil
}

func (a *RemoteAkasaLet) getComponent(name string) (*component, error) {
	c, ok := a.componentsByName[name]
	if !ok {
		return nil, fmt.Errorf("component %q not registered", name)
	}

	return c, nil
}

func (a *RemoteAkasaLet) addHandlers(hm *call.HandlerMap, c *component) {
	for i := range c.reg.Iface.NumMethod() {
		methodName := c.reg.Iface.Method(i).Name
		handler := func(ctx context.Context, args []byte) ([]byte, error) {
			if _, err := a.GetImpl(c.reg.Impl); err != nil {
				return nil, err
			}
			fn := c.serverStub.GetHandleFn(methodName)
			return fn(ctx, args)
		}
		hm.Set(c.reg.Name, methodName, handler)
	}
}

func (a *RemoteAkasaLet) Wait() error {
	return a.servers.Wait()
}

var _ control.AkasaletControl = (*RemoteAkasaLet)(nil)

type server struct {
	net.Listener
	alet *RemoteAkasaLet
}

var _ call.Listener = (*server)(nil)

func (s *server) Accept() (net.Conn, *call.HandlerMap, error) {
	conn, err := s.Listener.Accept()
	if err != nil {
		return nil, nil, err
	}

	hm, err := s.handlers(maps.Keys(s.alet.componentsByName))
	if err != nil {
		return nil, nil, err
	}

	return conn, hm, err
}

func (s *server) handlers(components []string) (*call.HandlerMap, error) {
	hm := call.NewHandlerMap()
	for _, comp := range components {
		c, err := s.alet.getComponent(comp)
		if err != nil {
			return nil, err
		}
		s.alet.addHandlers(hm, c)
	}

	return hm, nil
}

func parseEndpoints(addrList []string) ([]call.Endpoint, error) {
	var endpoints []call.Endpoint
	var err error
	var ep call.Endpoint
	for _, addr := range addrList {
		if ep, err = call.ParseNetEndpoint(addr); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, nil
}
