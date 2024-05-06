package envelope

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/kanengo/akasar/internal/errgroup"

	"github.com/kanengo/akasar/runtime/version"

	"github.com/kanengo/akasar/internal/net/call"

	"github.com/kanengo/akasar/runtime/codegen"

	"github.com/kanengo/akasar/internal/control"

	"github.com/kanengo/akasar/runtime/protomsg"

	"github.com/kanengo/akasar/runtime/deployers"

	"github.com/kanengo/akasar/runtime"

	"go.opentelemetry.io/otel/trace"

	"github.com/kanengo/akasar/runtime/protos"

	_ "github.com/kanengo/akasar"
)

type Handler interface {
	ActivateComponent(ctx context.Context, req *protos.ActivateComponentRequest) (*protos.ActivateComponentReply, error)
	LogBatch(ctx context.Context, batch *protos.LogEntryBatch) error
}

type Envelope struct {
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *slog.Logger
	tmpDir       string               //临时目录
	tmpDirOwned  bool                 //是否临时目录的创建者
	myUds        string               //unix domain socket
	akasalet     *protos.AkasaletArgs //akasalet 参数
	akasaletAddr string               //  akasalet 的监听地址
	config       *protos.AppConfig
	child        Child

	akasaletCtrl control.AkasaletControl

	metricsMu sync.Mutex
}

type Options struct {
	TempDir string
	Logger  *slog.Logger
	Tracer  trace.Tracer
	Child   Child
}

func NewEnvelope(ctx context.Context, aletArgs *protos.AkasaletArgs, config *protos.AppConfig, options Options) (*Envelope, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
	}()

	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	var removeDir bool
	tmpDir := options.TempDir
	tmpDirOwned := false
	if options.TempDir == "" {
		var err error
		tmpDir, err = runtime.NewTempDir()
		if err != nil {
			return nil, err
		}
		tmpDirOwned = true
		runtime.OnExitSignal(func() {
			_ = os.RemoveAll(tmpDir)
		})
		removeDir = true
		defer func() {
			if removeDir {
				_ = os.RemoveAll(tmpDir)
			}
		}()
	}

	myUds := deployers.NewUnixSocketPath(tmpDir)

	aletArgs = protomsg.Clone(aletArgs)
	aletArgs.ControlSocket = deployers.NewUnixSocketPath(tmpDir)
	aletArgs.Redirects = []*protos.AkasaletArgs_Redirect{
		{
			Component: control.DeployerPath,
			Target:    control.DeployerPath,
			Address:   "unix://" + myUds,
		},
	}

	e := &Envelope{
		ctx:          ctx,
		cancel:       cancel,
		logger:       options.Logger,
		tmpDir:       tmpDir,
		tmpDirOwned:  tmpDirOwned,
		myUds:        myUds,
		akasalet:     aletArgs,
		config:       config,
		akasaletCtrl: nil,
	}

	child := options.Child
	if child == nil {
		child = &ProcessChild{}
	}
	if err := child.Start(ctx, e.config, e.akasalet); err != nil {
		return nil, fmt.Errorf("NewEnvelope: %w", err)
	}

	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	aletCtrlStub, err := getAkasaletControlStub(ctx, aletArgs.ControlSocket, options)
	if err != nil {
		return nil, err
	}
	e.akasaletCtrl = aletCtrlStub

	//e.logger.Debug("envelope init akasalet start", "args", e.akasalet)
	reply, err := aletCtrlStub.InitAkasalet(e.ctx, &protos.InitAkasaletRequest{
		Sections: config.Sections,
	})
	if err != nil {
		e.logger.Error("envelop init akasar", "err", err)
		return nil, fmt.Errorf("NewEnvelope: %w", err)
	}
	//e.logger.Debug("envelope init akasalet", "dialAddr", reply.DialAddr, "version", reply.Version)
	e.logger.Debug("envelope init akasalet", slog.Any("reply", reply))
	if err := verifyAkasaletInfo(reply); err != nil {
		return nil, err
	}
	e.akasaletAddr = reply.DialAddr

	e.child = child
	removeDir = false
	cancel = func() {} // 延迟真正的 context cancellation

	return e, nil
}

func (e *Envelope) Serve(h Handler) error {
	e.logger.Debug("envelope serve", "socket", e.myUds, "args", e.akasalet)
	if e.tmpDirOwned {
		defer func() {
			_ = os.RemoveAll(e.tmpDir)
		}()
	}

	unixLis, err := net.Listen("unix", e.myUds)
	if err != nil {
		return err
	}

	var running errgroup.Group

	var stopErr error
	var once sync.Once

	// 发生错误， stop the envelope.
	stop := func(err error) {
		once.Do(func() {
			stopErr = err
		})
		e.cancel()
	}

	// Capture stdout and stderr from the akasalet.
	if stdout := e.child.Stdout(); stdout != nil {
		running.Go(func() error {
			err := e.logLines("stdout", stdout, h)
			stop(err)
			return nil
		})
	}
	if stderr := e.child.Stderr(); stderr != nil {
		running.Go(func() error {
			err := e.logLines("stderr", stderr, h)
			stop(err)
			return err
		})
	}

	// 启动 goroutine 等待context 取消
	running.Go(func() error {
		<-e.ctx.Done()
		err := e.ctx.Err()
		stop(err)
		return err
	})

	// 启动 goroutine 处理 deployer control的调用
	running.Go(func() error {
		err := deployers.ServeComponents(e.ctx, unixLis, e.logger, map[string]any{
			control.DeployerPath: h,
		})
		stop(err)
		return err
	})

	// 等待 goroutine 组结束
	_ = running.Wait()

	stop(e.child.Wait())

	return stopErr
}

// logLines 记录日志
func (e *Envelope) logLines(component string, src io.Reader, h Handler) error {
	entry := &protos.LogEntry{
		App:       e.akasalet.App,
		Version:   e.akasalet.DeploymentId,
		Component: component,
		Node:      e.akasalet.Id,
		Level:     component, // stdout or stderr
		File:      "",
		Line:      -1,
	}
	batch := &protos.LogEntryBatch{}
	batch.Entries = append(batch.Entries, entry)

	rdr := bufio.NewReader(src)
	for {
		line, err := rdr.ReadBytes('\n')
		if len(line) > 0 {
			entry.Msg = string(dropNewLine(line))
			entry.TimeMicros = 0
			if err := h.LogBatch(e.ctx, batch); err != nil {
				return err
			}
		}
		if err != nil {
			return fmt.Errorf("capture %s: %w", component, err)
		}
	}

}

func dropNewLine(line []byte) []byte {
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	return line
}

func verifyAkasaletInfo(alet *protos.InitAkasaletReply) error {
	if alet == nil {
		return fmt.Errorf("the first message from the akasalet must contain akasalet info")
	}

	if alet.DialAddr == "" {
		return fmt.Errorf("empty dial address for the akasalet")
	}

	if err := checkVersion(alet.Version); err != nil {
		return err
	}

	return nil
}

func checkVersion(v *protos.SemVer) error {
	if v == nil {
		return fmt.Errorf("nil app version")
	}
	got := version.SemVer{Major: int(v.Major), Minor: int(v.Minor), Patch: int(v.Patch)}

	if got != version.DeployerVersion {
		return fmt.Errorf("version mismatch: deployer's API verion %s is incompatible"+
			"with app's deployer API version %s", version.DeployerVersion, got)
	}

	return nil
}

// getAkasaletControlStub 返回一个调用 akasalet control 的stub
func getAkasaletControlStub(ctx context.Context, socket string, options Options) (control.AkasaletControl, error) {
	controllerReg, ok := codegen.Find(control.AkasaletPath)
	if !ok {
		return nil, fmt.Errorf("no controller registered for (%s)", control.AkasaletPath)
	}

	controlEndpoint := call.Unix(socket)
	resolver := call.NewConstantResolver(controlEndpoint)

	opts := call.ClientOptions{
		Logger: options.Logger,
	}

	conn, err := call.Connect(ctx, resolver, opts)
	if err != nil {
		return nil, err
	}

	stub := call.NewStub(control.AkasaletPath, controllerReg, conn, 0)
	obj := controllerReg.ClientStubFn(stub, "envelope", options.Tracer)

	return obj.(control.AkasaletControl), nil
}

func (e *Envelope) UpdateComponents(components []string) error {
	req := &protos.UpdateComponentsRequest{
		Components: components,
	}

	_, err := e.akasaletCtrl.UpdateComponents(context.TODO(), req)
	return err
}

func (e *Envelope) Pid() (int, bool) {
	return e.child.Pid()
}

func (e *Envelope) AkasaletAddress() string {
	return e.akasaletAddr
}

func (e *Envelope) GetHealth(ctx context.Context) protos.HealthStatus {
	reply, err := e.akasaletCtrl.GetHealth(ctx, &protos.GetHealthRequest{})
	if err != nil {
		return protos.HealthStatus_UNKNOWN
	}

	return reply.Status
}

func (e *Envelope) UpdateRoutingInfo(ctx context.Context, routing *protos.RoutingInfo) error {
	req := &protos.UpdateRoutingInfoRequest{RoutingInfo: routing}

	_, err := e.akasaletCtrl.UpdateRoutingInfo(ctx, req)

	return err
}
