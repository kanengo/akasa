package call

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/codes"

	"go.opentelemetry.io/otel/trace"

	"github.com/kanengo/akasar/runtime/logging"

	"github.com/kanengo/akasar/runtime/retry"
)

const (
	// 每一个message的请求头size
	msgHeaderSize = 16 + 8 + traceHeaderLen // method_key + deadline + trace_context
)

// Connection 发送rpc的连接
type Connection interface {
	Call(context.Context, MethodKey, []byte, CallOptions) ([]byte, error)

	Close()
}

type Listener interface {
	Accept() (net.Conn, *HandlerMap, error)
	Close() error
	Addr() net.Addr
}

// reconnectionConnection
// 连接关闭后自动重连
type reconnectionConnection struct {
	opts ClientOptions

	mu     sync.Mutex
	conns  map[string]*clientConnection
	closed bool

	resolver       Resolver
	cancelResolver func()
	resolverDone   sync.WaitGroup
}

func (rc *reconnectionConnection) Call(ctx context.Context, key MethodKey, bytes []byte, opts CallOptions) ([]byte, error) {
	if !opts.Retry {
		return rc.callOnce(ctx, key, bytes, opts)
	}
	for r := retry.Begin(); r.Continue(ctx); {
		res, err := rc.callOnce(ctx, key, bytes, opts)
		if err != nil {
			if errors.Is(err, Unreachable) || errors.Is(err, CommunicationError) {
				continue
			}
			rc.opts.Logger.Error("callOnce", "err", err)
		}
		return res, err
	}
	return nil, ctx.Err()
}

func (rc *reconnectionConnection) callOnce(ctx context.Context, key MethodKey, args []byte, opts CallOptions) ([]byte, error) {
	// 消息头
	var hdr [msgHeaderSize]byte
	copy(hdr[0:], key[:])

	deadline, haveDeadline := ctx.Deadline()
	if haveDeadline {
		micros := time.Until(deadline).Microseconds()
		if micros <= 0 {
			//已经超时，不用去发送请求了
			<-ctx.Done()
			return nil, ctx.Err()
		}
		binary.LittleEndian.PutUint64(hdr[16:], uint64(micros))
	}

	writeTraceContext(ctx, hdr[24:])

	rpc := &call{
		doneSignal: make(chan struct{}),
	}

	conn, nc, err := rc.startCall(ctx, rpc, opts)
	if err != nil {
		return nil, err
	}

	if err := writeMessage(nc, &conn.wLock, requestMessage, rpc.id, hdr[:], args, rc.opts.WriteFlattenLimit); err != nil {
		// 关闭连接(通过重连进行尝试恢复)， 结束所有正在进行中的请求
		conn.shutdown("client send request", err)
		// 结束请求
		conn.endCall(rpc)
		return nil, fmt.Errorf("%w: %s", CommunicationError, err)
	}

	if rc.opts.OptimisticSpinDuration > 0 {
		// 通过自转等待响应返回
		for start := time.Now(); time.Since(start) < rc.opts.OptimisticSpinDuration; {
			if atomic.LoadUint32(&rpc.done) > 0 {
				return rpc.response, rpc.err
			}
		}
	}

	if cDone := ctx.Done(); cDone != nil {
		select {
		case <-rpc.doneSignal:
		case <-cDone:
			// Canceled or deadline expired.
			conn.endCall(rpc)
			if !haveDeadline || time.Now().Before(deadline) {
				//未超时就已经被取消,通知服务器
				if err := writeMessage(nc, &conn.wLock, cancelMessage, rpc.id, nil, nil, rc.opts.WriteFlattenLimit); err != nil {
					conn.shutdown("client send cancel", err)
				}
			}
			return nil, ctx.Err()
		}
	} else {
		<-rpc.doneSignal
	}

	//rc.opts.Logger.Debug("rpc.response", "res", rpc.response, "err", rpc.err)

	return rpc.response, rpc.err
}

func (rc *reconnectionConnection) Close() {
	func() {
		rc.mu.Lock()
		defer rc.mu.Unlock()
		if rc.closed {
			return
		}
		rc.closed = true
		for _, conn := range rc.conns {
			conn.close()
		}
	}()
	rc.cancelResolver()
	rc.resolverDone.Wait()
}

func (rc *reconnectionConnection) updateEndpoints(ctx context.Context, endpoints []Endpoint) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.closed {
		return fmt.Errorf("updateEndpoints on closed connection")
	}

	keep := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		addr := ep.Address()
		keep[addr] = struct{}{}
		if _, ok := rc.conns[addr]; !ok {
			// New endpoint, create connection and manage it.
			ctx, cancel := context.WithCancel(ctx)
			c := &clientConnection{
				rc:       rc,
				cancel:   cancel,
				logger:   rc.opts.Logger,
				endpoint: ep,
				calls:    map[uint64]*call{},
				lastID:   0,
			}
			rc.conns[addr] = c
			c.register()
			go c.manage(ctx)
		}
	}

	for addr, c := range rc.conns {
		if _, ok := keep[addr]; ok {
			// 仍然存货，继续保持
			continue
		}
		c.unregister()
	}

	return nil
}

func (rc *reconnectionConnection) watchResolver(ctx context.Context, v *Version) {
	defer rc.resolverDone.Done()

	for r := retry.Begin(); r.Continue(ctx); {
		endpoints, newVersion, err := rc.resolver.Resolve(ctx, v)
		if err != nil {
			logError(rc.opts.Logger, "watchResolver", err)
			continue
		}

		if newVersion == nil {
			logError(rc.opts.Logger, "watchResolver", errors.New("non-constant resolver returned a nil version"))
			continue
		}

		if *v == *newVersion {
			continue
		}

		if err := rc.updateEndpoints(ctx, endpoints); err != nil {
			logError(rc.opts.Logger, "watchResolver", err)
		}
		v = newVersion
		r.Reset()
	}
}

func (rc *reconnectionConnection) startCall(ctx context.Context, rpc *call, opts CallOptions) (*clientConnection, net.Conn, error) {
	for r := retry.Begin(); r.Continue(ctx); {
		rc.mu.Lock()
		if rc.closed {
			rc.mu.Unlock()
			return nil, nil, fmt.Errorf("call on a closed connection")
		}

		replica, ok := rc.opts.Balancer.Pick(opts)
		if !ok {
			rc.mu.Unlock()
			continue
		}

		c, ok := replica.(*clientConnection)
		if !ok {
			rc.mu.Unlock()
			return nil, nil, fmt.Errorf("internal error: wrong connection type %#v returned by load balancer", replica)
		}

		c.lastID++
		rpc.id = c.lastID
		c.calls[rpc.id] = rpc
		c.callStart()
		nc := c.c
		rc.mu.Unlock()

		return c, nc, nil
	}

	return nil, nil, ctx.Err()
}

type connState int8

const (
	missing      connState = iota
	disconnected           //断开的连接   //
	checking
	idle
	active
	draining
)

var connStateNames = []string{
	"missing",
	"disconnected",
	"checking",
	"idle",
	"active",
	"draining",
}

func (s connState) String() string {
	return connStateNames[s]
}

// clientConnection 管理一个网络连接
type clientConnection struct {
	rc     *reconnectionConnection
	cancel func()

	logger   *slog.Logger
	endpoint Endpoint

	wLock sync.Mutex

	state          connState
	loggedShutdown bool
	inBalancer     bool //是否已在balancer注册
	c              net.Conn
	cBuf           *bufio.Reader
	version        version
	calls          map[uint64]*call // 正在进行中的call
	lastID         uint64
}

func (c *clientConnection) Address() string {
	return c.endpoint.Address()
}

func (c *clientConnection) register() {
	switch c.state {
	case missing: //连接已删除,则设置为断开连接，等待重新连接
		c.setState(disconnected)
	case draining: //连接正在断开，则回复为激活
		c.setState(active)
	default:
	}
}

func (c *clientConnection) unregister() {
	switch c.state {
	case disconnected, checking, idle: // 未连接、检测中、闲置的，直接删除
		c.setState(missing)
	case active:
		c.setState(draining) // 正在活动的连接，设置为正在断开
	default:

	}
}

// manage 处理一个存活的连接直到missing
// 新建连接时执行
func (c *clientConnection) manage(ctx context.Context) {
	for r := retry.Begin(); r.Continue(ctx); {
		if c.connectOnce(ctx) {
			// 连接发生错误重连
			r.Reset()
		}
	}
}

// setState 连接状态管理
func (c *clientConnection) setState(s connState) {
	// ide <-> active 转换可能会发生多次， 不记录日志
	if c.state == active && s == idle {
		c.state = idle
		if len(c.calls) != 0 {
			panic(fmt.Sprintf("%v connection: wrong number of calls %d", s, len(c.calls)))
		}
		return
	} else if c.state == idle && s == active {
		c.state = active
		if len(c.calls) == 0 {
			panic(fmt.Sprintf("%v connection: wrong number of calls %d", s, len(c.calls)))
		}
		return
	}

	c.logger.Debug("connection", "addr", c.endpoint.Address(), "from", c.state, "to", s)
	c.state = s

	// 连接丢失, 删除连接
	if s == missing {
		delete(c.rc.conns, c.endpoint.Address())
		if c.cancel != nil {
			c.cancel()
			c.cancel = nil
		}
	} // else: 调用方负责把 c 添加到 rc

	if s == missing || s == disconnected {
		if c.c != nil {
			_ = c.c.Close()
			c.c = nil
			c.cBuf = nil
		}
	} // else: 调用方负责设置 c.c 和 c.cBuf

	if s == idle || s == active {
		if !c.inBalancer {
			c.rc.opts.Balancer.Add(c)
			c.inBalancer = true
		}
	} else {
		if c.inBalancer {
			c.rc.opts.Balancer.Remove(c)
			c.inBalancer = false
		}
	}

	// 处理进行中的 calls
	if s == active || s == draining {
		// 保持 call live
	} else {
		c.endCalls(fmt.Errorf("%s:%v", "communication error", s))
	}

	c.checkInvariants()
}

func (c *clientConnection) endCalls(err error) {
	for id, calling := range c.calls {
		calling.err = err
		atomic.StoreUint32(&calling.done, 1)
		close(calling.doneSignal)
		delete(c.calls, id)
	}
}

func (c *clientConnection) close() {
	c.endCalls(fmt.Errorf("communication error: connection closed"))
	c.setState(missing)
}

// checkInvariants 检查连接状态与数据是否符合
func (c *clientConnection) checkInvariants() {
	s := c.state

	// 连接的存在与否 与 missing 状态不符
	if _, ok := c.rc.conns[c.endpoint.Address()]; ok != (s != missing) {
		panic(fmt.Sprintf("%v connection: wrong connection table presence %v", s, ok))
	}

	// 已连接上的连接， 状态却不是 checking/idle/active/draining 之一
	if (c.c != nil) != (s == checking || s == idle || s == active || s == draining) {
		panic(fmt.Sprintf("%v connection: wrong net.Conn %v", s, c.c))
	}

	// 连接负载均衡注册状况与状态不符
	if c.inBalancer != (s == idle || s == active) {
		panic(fmt.Sprintf("%v connection: wrong balancer presence %v", s, c.inBalancer))
	}

	// 正在进行中的call与状态不符合 ，只有active和draining 才能有正在进行的连接
	if (len(c.calls) != 0) != (s == active || s == draining) {
		panic(fmt.Sprintf("%v connection: wrong connection table presence", s))
	}
}

// connectOnce 建立一次连接并管理
// 返回 true, 如果连接通信成功
func (c *clientConnection) connectOnce(ctx context.Context) bool {
	nc, err := c.endpoint.Dial(ctx)
	if err != nil {
		logError(c.logger, "dial", err)
		return false
	}
	defer func() {
		_ = nc.Close()
	}()

	c.rc.mu.Lock()
	defer c.rc.mu.Unlock()

	c.c = nc
	c.cBuf = bufio.NewReader(nc)
	c.loggedShutdown = false
	c.connected()

	if err := c.exchangeVersions(); err != nil {
		return false
	}
	c.checked()

	for c.state == idle || c.state == active || c.state == draining {
		if err := c.readAndProcessMessage(); err != nil {
			c.fail("client read", err)
		}
	}

	return true
}

func (c *clientConnection) connected() {
	switch c.state {
	case disconnected:
		c.setState(checking)
	default:

	}
}

func (c *clientConnection) checked() {
	switch c.state {
	case checking:
		c.setState(idle)
	default:

	}
}

func (c *clientConnection) callStart() {
	switch c.state {
	case idle:
		c.setState(active)
	default:

	}
}

// lastDone 当前的最后一个 calling 已经完成
func (c *clientConnection) lastDone() {
	switch c.state {
	case active:
		c.setState(idle)
	case draining:
		c.setState(missing)
	default:

	}
}

// exchangeVersions sends client version to server and waits for the server version.
func (c *clientConnection) exchangeVersions() error {
	nc, buf := c.c, c.cBuf

	// 读取网络数据时不持有锁
	c.rc.mu.Unlock()
	defer c.rc.mu.Lock()

	if err := writeVersion(nc, &c.wLock); err != nil {
		return err
	}
	mt, id, msg, err := readMessage(buf)
	if err != nil {
		return err
	}
	if mt != versionMessage {
		return fmt.Errorf("unexpected message type %d, expecting %d", mt, versionMessage)
	}
	v, err := getVersion(id, msg)
	if err != nil {
		return err
	}

	c.version = v

	return nil
}

// readAndProcessMessage
func (c *clientConnection) readAndProcessMessage() error {
	buf := c.cBuf

	// 网络操作期间不持有锁
	c.rc.mu.Unlock()
	defer c.rc.mu.Lock()

	mt, id, msg, err := readMessage(buf)
	if err != nil {
		return err
	}

	switch mt {
	case versionMessage:
		_, err := getVersion(id, msg)
		if err != nil {
			return err
		}
		// 初次握手后，忽略版本消息类型
	case responseMessage, responseError:
		calling := c.findAndEndCall(id)
		if calling == nil {
			return nil
		}

		if mt == responseError {
			if err, ok := decodeError(msg); ok {
				calling.err = err
			} else {
				calling.err = fmt.Errorf("%s: could not decode error", "communication error")
			}
		} else {
			calling.response = msg
		}
		atomic.StoreUint32(&calling.done, 1)
		close(calling.doneSignal)
	default:
		return fmt.Errorf("invalid response %d", mt)
	}

	return nil
}

// findAndEndCall 找到正在进行的调用并结束
func (c *clientConnection) findAndEndCall(id uint64) *call {
	c.rc.mu.Lock()
	defer c.rc.mu.Unlock()

	calling := c.calls[id]
	if calling != nil {
		delete(c.calls, id)
		if len(c.calls) == 0 {
			c.lastDone()
		}
	}

	return calling
}

func (c *clientConnection) fail(details string, err error) {
	if !c.loggedShutdown {
		c.loggedShutdown = true
		logError(c.logger, details, err)
	}

	c.endCalls(fmt.Errorf("%s: %s:%s", "communication error", details, err))

	switch c.state {
	case checking, idle, active:
		c.setState(disconnected)
	case draining:
		c.setState(missing)
	default:

	}
}

// shutdown 当在操作连接中发现错误时，关闭网络连接和取消所有进行中的请求
func (c *clientConnection) shutdown(details string, err error) {
	c.rc.mu.Lock()
	defer c.rc.mu.Unlock()
	c.fail(details, err)
}

func (c *clientConnection) endCall(rpc *call) {
	c.rc.mu.Lock()
	defer c.rc.mu.Unlock()
	delete(c.calls, rpc.id)
	if len(c.calls) == 0 {
		c.lastDone()
	}
}

type call struct {
	id         uint64
	doneSignal chan struct{}

	err      error
	response []byte

	done uint32
}

func Connect(ctx context.Context, resolver Resolver, opts ClientOptions) (Connection, error) {
	conn := reconnectionConnection{
		opts:           opts.withDefaults(),
		conns:          make(map[string]*clientConnection),
		resolver:       resolver,
		cancelResolver: func() {},
	}

	endpoints, version, err := resolver.Resolve(ctx, nil)
	if err != nil {
		return nil, err
	}

	// resolver 不可变且没有任何endpoint
	if resolver.IsConstant() && len(endpoints) == 0 {
		return nil, fmt.Errorf("%s: no endpoints available", "unreachable")
	}

	// resolver 可变但 version为nil
	if !resolver.IsConstant() && version == nil {
		return nil, fmt.Errorf("non-constant resolver returned a nil version")
	}

	if err := conn.updateEndpoints(ctx, endpoints); err != nil {
		return nil, err
	}

	// 如果 resolver 时可变得，则启动一个goroutine监听endpoints的更新
	if !resolver.IsConstant() {
		ctx, cancel := context.WithCancel(ctx)
		conn.cancelResolver = cancel
		conn.resolverDone.Add(1)
		go conn.watchResolver(ctx, version)
	}

	return &conn, nil
}

func logError(logger *slog.Logger, details string, err error) {
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.ErrClosedPipe) {
		logger.Info(details, "err", err)
	} else {
		logger.Error(details, "err", err)
	}
}

// serverConnection 管理服务端的连接
type serverConnection struct {
	opts        ServerOptions
	c           net.Conn
	cBuf        *bufio.Reader
	wLock       sync.Mutex
	mu          sync.Mutex
	closed      bool
	version     version
	cancelFuncs map[uint64]func()
}

// readRequests runs on the server side reading messages sent over a connection by the client.
func (c *serverConnection) readRequests(ctx context.Context, hmap *HandlerMap, onDone func()) {
	for ctx.Err() == nil {
		mt, id, msg, err := readMessage(c.cBuf)
		if err != nil {
			c.shutdown("server read", err)
			onDone()
			return
		}

		switch mt {
		case versionMessage:
			v, err := getVersion(id, msg)
			if err != nil {
				c.shutdown("server read version", err)
				onDone()
				return
			}

			c.mu.Lock()
			c.version = v
			c.mu.Unlock()

			// 响应我的版本
			if err := writeVersion(c.c, &c.wLock); err != nil {
				c.shutdown("server response version", err)
				onDone()
				return
			}
		case requestMessage:
			if c.opts.InlineHandlerDuration > 0 {
				// 内联执行handler, 如果规定时间没有相应，启动其他goroutine读取请求
				t := time.AfterFunc(c.opts.InlineHandlerDuration, func() {
					c.readRequests(ctx, hmap, onDone)
				})
				c.runHandler(hmap, id, msg)
				if !t.Stop() {
					// 有其他goroutine在处理请求， 退出本方法
					return
				}
			} else {
				go c.runHandler(hmap, id, msg)
			}
		case cancelMessage:
			c.endRequest(id)
		default:

		}
	}
	_ = c.c.Close()
	onDone()
}

// shutdown 关闭连接，实行所有cancel方法
func (c *serverConnection) shutdown(details string, err error) {
	_ = c.c.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		logError(c.opts.Logger, "shutdown:"+details, err)
	}

	for id, cf := range c.cancelFuncs {
		cf()
		delete(c.cancelFuncs, id)
	}
}

// runHandler 执行对应的handler
func (c *serverConnection) runHandler(hmap *HandlerMap, id uint64, msg []byte) {
	if len(msg) < msgHeaderSize {
		c.shutdown("server handler", fmt.Errorf("missing request header"))
	}

	// Extract method key.
	var mk MethodKey
	copy(mk[:], msg)

	methodName := hmap.names[mk] //component.method
	if methodName == "" {
		methodName = "handler"
	} else {
		methodName = logging.ShortenComponent(methodName)
	}

	// Extract trace context and create a new child span the method
	//
	ctx := context.Background()
	span := trace.SpanFromContext(ctx) // noop ctx
	if sc := readTraceContext(msg[24:]); sc.IsValid() {
		ctx, span = c.opts.Tracer.Start(trace.ContextWithSpanContext(ctx, sc), methodName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
	}

	// Add deadline information from the header to the context.
	micro := binary.LittleEndian.Uint64(msg[16:])
	var cancelFunc func()
	if micro != 0 {
		deadline := time.Now().Add(time.Microsecond * time.Duration(micro))
		ctx, cancelFunc = context.WithDeadline(ctx, deadline)
	} else {
		ctx, cancelFunc = context.WithCancel(ctx)
	}
	defer func() {
		if cancelFunc != nil {
			cancelFunc()
		}
	}()

	payload := msg[msgHeaderSize:]
	var err error
	var result []byte

	fn, ok := hmap.handlers[mk]
	if !ok {
		err = fmt.Errorf("unknown method: %s", methodName)
	} else {
		if err := c.startRequest(id, cancelFunc); err != nil {
			// 连接已关闭
			logError(c.opts.Logger, "handle "+hmap.names[mk], err)
			return
		}
		cancelFunc = nil // endRequest 或 cancellation 会处理
		defer c.endRequest(id)
		result, err = fn(ctx, payload)
	}

	mt := responseMessage
	if err != nil {
		mt = responseError
		result = encodeError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	if err := writeMessage(c.c, &c.wLock, mt, id, nil, result, c.opts.WriteFlattenLimit); err != nil {
		c.shutdown("server write"+hmap.names[mk], err)
	}

}

func (c *serverConnection) startRequest(id uint64, cancelFunc func()) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("startRequest: %w", net.ErrClosed)
	}

	c.cancelFuncs[id] = cancelFunc

	return nil
}

func (c *serverConnection) endRequest(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cancelFunc, ok := c.cancelFuncs[id]; ok {
		delete(c.cancelFuncs, id)
		cancelFunc()
	}
}

// serverState 管理服务端所有连接
type serverState struct {
	opts  ServerOptions
	mu    sync.Mutex
	conns map[*serverConnection]struct{}
}

func (ss *serverState) stop() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for c := range ss.conns {
		_ = c.c.Close()
	}
}

func (ss *serverState) serveConnection(ctx context.Context, conn net.Conn, hmap *HandlerMap) {
	c := &serverConnection{
		opts:        ss.opts,
		c:           conn,
		cBuf:        bufio.NewReader(conn),
		version:     initialVersion,
		cancelFuncs: map[uint64]func(){},
	}
	ss.register(c)

	go c.readRequests(ctx, hmap, func() { ss.unregister(c) })
}

func (ss *serverState) register(c *serverConnection) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.conns == nil {
		ss.conns = make(map[*serverConnection]struct{})
	}
	ss.conns[c] = struct{}{}
}

func (ss *serverState) unregister(c *serverConnection) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.conns, c)
}

// Serve 开始监听连接
func Serve(ctx context.Context, l Listener, opts ServerOptions) error {
	opts = opts.withDefaults()
	ss := &serverState{opts: opts}
	defer ss.stop()

	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, hmap, err := l.Accept()
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			_ = l.Close()
			return fmt.Errorf("call server error listening on %s: %w", l.Addr(), err)
		}
		ss.serveConnection(ctx, conn, hmap)
	}
}
