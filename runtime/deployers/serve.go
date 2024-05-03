package deployers

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"sync"

	"github.com/kanengo/akasar/internal/net/call"
)

type fixedListener struct {
	net.Listener
	handlers *call.HandlerMap
}

func (f *fixedListener) Accept() (net.Conn, *call.HandlerMap, error) {
	c, err := f.Listener.Accept()
	return c, f.handlers, err
}

// ServeComponents 启动处理调用组件方法请求服务
func ServeComponents(ctx context.Context, listener net.Listener, logger *slog.Logger, components map[string]any) error {
	handlers := call.NewHandlerMap()

	for component, impl := range components {
		if err := handlers.AddHandlers(component, impl); err != nil {
			return err
		}
	}

	f := &fixedListener{
		Listener: listener,
		handlers: handlers,
	}

	return call.Serve(ctx, f, call.ServerOptions{
		Logger: logger,
	})
}

var (
	pathMu      sync.Mutex
	pathCounter int64
)

func NewUnixSocketPath(dir string) string {
	pathMu.Lock()
	defer pathMu.Unlock()

	pathCounter++
	return filepath.Join(dir, fmt.Sprintf("_uds%d", pathCounter))
}
