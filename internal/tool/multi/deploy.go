package multi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/kanengo/akasar/internal/tool/config"

	"github.com/kanengo/akasar/runtime/codegen"

	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/tool"
)

const (
	ConfigKey      = "github.com/kanengo/akasar/multi"
	ConfigShortKey = "multi"
)

var deployCmd = tool.Command{
	Name:        "deploy",
	Description: "Deploy Service Akasar app on multi machine communicate by ipc",
	Help:        "",
	Fn:          deploy,
}

func deploy(ctx context.Context, args []string) error {
	// Validate command line arguments.
	if len(args) == 0 {
		return fmt.Errorf("no config file provided")
	}
	if len(args) > 1 {
		return fmt.Errorf("too many arguments")
	}

	configFile := args[0]
	bytes, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("load config file %q: %w", configFile, err)
	}

	appConfig, err := runtime.ParseConfig(configFile, string(bytes), codegen.ComponentConfigValidator)
	if err != nil {
		return fmt.Errorf("parse config file %q: %w", configFile, err)
	}
	if _, err := os.Stat(appConfig.Binary); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("binary %q does not exist", appConfig.Binary)
	}

	localConfig, err := config.GetDeployerConfig[MultiConfig, MultiConfig_ListenerOptions](ConfigKey, ConfigShortKey, appConfig)
	if err != nil {
		return err
	}
	localConfig.App = appConfig

	tmpDir, err := runtime.NewTempDir()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	runtime.OnExitSignal(func() {
		_ = os.RemoveAll(tmpDir)
	})

	deploymentId := gonanoid.Must(16)
	d, err := newDeployer(ctx, deploymentId, localConfig, tmpDir)

	// Run a status server.
	statusLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	mux := http.NewServeMux()
	// TODO(leeka) register status handler
	go func() {
		if err := serveHTPP(ctx, statusLis, mux); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "status server :%v\n", err)
		}
	}()

	if err := d.startRoot(); err != nil {
		return fmt.Errorf("start root process: %w", err)
	}

	return d.wait()
}

func serveHTPP(ctx context.Context, lis net.Listener, handler http.Handler) error {
	server := http.Server{Handler: handler}
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(lis)
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		return server.Shutdown(ctx)
	}
}
