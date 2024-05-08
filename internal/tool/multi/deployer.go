package multi

import (
	"context"
	"fmt"
	"github.com/kanengo/akasar/internal/net/call"
	"github.com/kanengo/akasar/runtime/deployers"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/kanengo/akasar/runtime/logging"

	"github.com/kanengo/akasar/internal/routing"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/kanengo/akasar/runtime/envelope"

	"golang.org/x/exp/maps"

	"github.com/kanengo/akasar/runtime/protos"

	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/errgroup"

	"github.com/kanengo/akasar/runtime/traces"
)

const (
	defaultReplication = 1
)

type deployer struct {
	ctx          context.Context
	cancel       context.CancelFunc
	deploymentId string
	tmpDir       string
	config       *MultiConfig
	running      errgroup.Group
	started      time.Time
	logger       *slog.Logger
	traceDB      *traces.DB
	groups       map[string]*group

	mu  sync.Mutex
	err error
}

type proxyInfo struct {
	listener string
}

func (d *deployer) LogBatch(ctx context.Context, batch *protos.LogEntryBatch) error {
	pp := logging.NewPrettyPrinter()
	for _, entry := range batch.Entries {
		if entry.Level == slog.LevelError.String() {
			_, _ = fmt.Fprintln(os.Stderr, pp.Format(entry))
		} else {
			_, _ = fmt.Fprintln(os.Stderr, pp.Format(entry))
		}
	}

	return nil
}

func (d *deployer) HandlerTraceSpans(ctx context.Context, spans *protos.TraceSpans) error {
	return d.traceDB.Store(ctx, d.config.App.Name, d.deploymentId, spans)
}

func (d *deployer) GetListenerAddress(ctx context.Context, request *protos.GetListenerAddressRequest) (*protos.GetListenerAddressReply, error) {
	return &protos.GetListenerAddressReply{Address: "localhost:0"}, nil
}

func (d *deployer) ExportListener(ctx context.Context, request *protos.ExportListenerRequest) (*protos.ExportListenerReply, error) {
	return &protos.ExportListenerReply{}, nil
}

// group 包含一组组件的信息
type group struct {
	name        string
	pids        []int64
	envelopes   []*envelope.Envelope
	started     map[string]bool
	addresses   map[string]bool
	assignments map[string]*protos.Assignment
	callable    []string

	subscribers map[string][]*envelope.Envelope //订阅组件变化
}

// routing 返回组件的路由信息
func (g *group) routing(component string) *protos.RoutingInfo {
	return &protos.RoutingInfo{
		Component:  component,
		Replicas:   maps.Keys(g.addresses),
		Assignment: g.assignments[component],
	}
}

func newDeployer(ctx context.Context, deploymentId string, config *MultiConfig, tmpDir string) (*deployer, error) {
	traceDB, err := traces.OpenDB(ctx, traceDBFile)
	if err != nil {
		return nil, fmt.Errorf("error opening trace DB: %v", err)
	}
	pp := logging.NewPrettyPrinter()
	logger := slog.New(logging.NewLogHandler(func(entry *protos.LogEntry) {
		_, _ = fmt.Fprintln(os.Stdout, pp.Format(entry))
	}, logging.Options{
		App:       config.App.Name,
		Component: "deployer",
		Akasalet:  gonanoid.Must(16),
		Attrs:     nil,
		Level:     int32(slog.LevelDebug),
	}))

	ctx, cancel := context.WithCancel(ctx)
	d := &deployer{
		ctx:          ctx,
		cancel:       cancel,
		deploymentId: deploymentId,
		tmpDir:       tmpDir,
		config:       config,
		running:      errgroup.Group{},
		started:      time.Now(),
		traceDB:      traceDB,
		logger:       logger,
	}

	if err := d.computeComponentGroups(); err != nil {
		return nil, err
	}

	call.InitUnifiedClientConnectionManager(ctx, logger)

	// Start a goroutine that watches for context cancelation.
	d.running.Go(func() error {
		<-d.ctx.Done()
		err := d.ctx.Err()
		d.stop(err)
		return err
	})

	return d, nil
}

// computeComponentGroups 根据配置组件分组
func (d *deployer) computeComponentGroups() error {
	groups := map[string]*group{}

	ensureGroup := func(component string) (*group, error) {
		if g, ok := groups[component]; ok {
			return g, nil
		}

		g := &group{
			name:        component,
			started:     map[string]bool{},
			addresses:   map[string]bool{},
			assignments: map[string]*protos.Assignment{},
			subscribers: map[string][]*envelope.Envelope{},
		}
		groups[component] = g
		return g, nil
	}

	for _, grp := range d.config.App.Colocate {
		if len(grp.Components) == 0 {
			continue
		}

		g, err := ensureGroup(grp.Components[0])
		if err != nil {
			return err
		}

		for i := 1; i < len(grp.Components); i++ {
			//若component再不同组有重复，则后面的会覆盖掉之前的组
			groups[grp.Components[i]] = g
		}
	}

	// TODO(leeka) check components valid in binary

	_, err := ensureGroup(runtime.Root)
	if err != nil {
		return err
	}

	d.groups = groups

	return nil
}

func (d *deployer) startRoot() error {
	return d.activeComponent(&protos.ActivateComponentRequest{
		Component: runtime.Root,
	})
}

// activeComponent 激活组件，通知组件所在组
func (d *deployer) activeComponent(req *protos.ActivateComponentRequest) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	target, ok := d.groups[req.Component] //组件组
	if !ok {
		return fmt.Errorf("unknown component %q", req.Component)
	}

	if !target.started[req.Component] { //组件在对应的组里还没启动
		target.started[req.Component] = true

		components := maps.Keys(target.started)
		for _, e := range target.envelopes {
			if err := e.UpdateComponents(components); err != nil {
				return err
			}
		}

		//初始化路由分配
		if req.Routed {
			replicas := maps.Keys(target.addresses)
			assignments := routingAlgo(&protos.Assignment{Version: 0}, replicas)
			target.assignments[req.Component] = assignments
		}

		routingInfo := target.routing(req.Component)
		for _, sub := range target.subscribers[req.Component] {
			if err := sub.UpdateRoutingInfo(context.TODO(), routingInfo); err != nil {
				return err
			}
		}
	}

	return d.startColocationGroup(target)
}

func (d *deployer) startColocationGroup(g *group) error {
	if d.err != nil {
		return d.err
	}

	if len(g.envelopes) == defaultReplication {
		return nil
	}

	components := maps.Keys(g.started)
	for range defaultReplication {
		info := &protos.AkasaletArgs{
			App:             d.config.App.Name,
			DeploymentId:    d.deploymentId,
			Id:              gonanoid.Must(16),
			RunMain:         g.started[runtime.Root],
			InternalAddress: call.Unix(deployers.NewUnixSocketPath(d.tmpDir)).Address(),
			LogLevel:        d.config.App.LogLevel,
		}

		e, err := envelope.NewEnvelope(d.ctx, info, d.config.App, envelope.Options{
			Logger: d.logger,
		})
		if err != nil {
			return err
		}

		h := &handler{
			deployer:   d,
			g:          g,
			envelope:   e,
			subscribed: map[string]bool{},
		}

		d.running.Go(func() error {
			err := e.Serve(h)
			d.stop(err)
			return err
		})

		pid, ok := e.Pid()
		if !ok {
			panic("multi deployer child must be a real process")
		}
		if err := d.registerReplica(g, e.AkasaletAddress(), pid); err != nil {
			return err
		}

		if err := e.UpdateComponents(components); err != nil {
			return err
		}

		g.envelopes = append(g.envelopes, e)
	}

	return nil
}

// routingAlgo 路由算法
func routingAlgo(currAssignment *protos.Assignment, replicas []string) *protos.Assignment {
	assignment := routing.EqualSlices(replicas)
	assignment.Version = currAssignment.Version + 1
	return assignment
}

// registerReplica 注册实例，更新路由信息
func (d *deployer) registerReplica(g *group, replicaAddr string, pid int) error {
	if g.addresses[replicaAddr] {
		return nil
	}
	g.addresses[replicaAddr] = true
	g.pids = append(g.pids, int64(pid))

	replicas := maps.Keys(g.addresses)
	for component, assignment := range g.assignments {
		assignment = routingAlgo(assignment, replicas)
		g.assignments[component] = assignment
	}

	// 通知订阅的组件
	for component := range g.started {
		routingInfo := g.routing(component)
		for _, sub := range g.subscribers[component] {
			if err := sub.UpdateRoutingInfo(d.ctx, routingInfo); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deployer) stop(err error) {
	d.mu.Lock()
	if d.err == nil {
		d.err = err
	}
	d.mu.Unlock()

	d.cancel()
}

func (d *deployer) wait() error {
	_ = d.running.Wait()
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// handler control.DeployerControl
type handler struct {
	*deployer
	g          *group
	envelope   *envelope.Envelope
	subscribed map[string]bool
}

var _ envelope.Handler = (*handler)(nil)

func (h *handler) ActivateComponent(ctx context.Context, req *protos.ActivateComponentRequest) (*protos.ActivateComponentReply, error) {
	if err := h.subscribedTo(req); err != nil {
		return nil, err
	}

	return &protos.ActivateComponentReply{}, h.activeComponent(req)
}

func (h *handler) subscribedTo(req *protos.ActivateComponentRequest) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.subscribed[req.Component]; ok {
		return nil
	}
	h.subscribed[req.Component] = true

	g, ok := h.groups[req.Component]
	if !ok {
		return fmt.Errorf("unknown component %q", req.Component)
	}

	if !req.Routed && h.g.name == g.name {
		// 没有路由器，组一样，路由到本地
		routingInfo := &protos.RoutingInfo{
			Component: req.Component,
			Local:     true,
		}
		return h.envelope.UpdateRoutingInfo(context.TODO(), routingInfo)
	}

	// 远程路由
	g.subscribers[req.Component] = append(g.subscribers[req.Component], h.envelope)

	return h.envelope.UpdateRoutingInfo(context.TODO(), g.routing(req.Component))
}
