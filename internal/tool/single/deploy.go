package single

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/kanengo/akasar/internal/tool"

	"github.com/kanengo/akasar/internal/tool/config"

	"github.com/kanengo/akasar/runtime"
	"github.com/kanengo/akasar/runtime/codegen"
)

const (
	ConfigKey      = "github.com/kanengo/akasar/single"
	ConfigShortKey = "single"
)

var deployCmd = tool.Command{
	Name:        "deploy",
	Description: "Deploy a single Service Akasar app",
	Help:        "",
	Fn:          deploy,
}

func deploy(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no deployerConfig file provided")
	}

	if len(args) > 1 {
		return fmt.Errorf("too many arguments provided")
	}

	configFile := args[0]
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("load deployerConfig file %q: %w", configFile, err)
	}

	app, err := runtime.ParseConfig(configFile, string(configBytes), codegen.ComponentConfigValidator)
	if err != nil {
		return fmt.Errorf("parse deployerConfig file %q: %w", configFile, err)
	}
	if _, err := os.Stat(app.Binary); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("binary %q does not exist", app.Binary)
	}

	deployerConfig, err := config.GetDeployerConfig[SingleConfig, SingleConfig_ListenerOptions](ConfigKey, ConfigShortKey, app)
	if err != nil {
		return fmt.Errorf("get deploy deployerConfig: %w", err)
	}
	deployerConfig.App = app

	cmd := exec.Command(app.Binary, app.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Environ(), "SERVICEAKASAR_CONFIG="+configFile)

	killed := make(chan os.Signal, 1)
	signal.Notify(killed, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-killed
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		os.Exit(1)
	}()

	return cmd.Run()
}
