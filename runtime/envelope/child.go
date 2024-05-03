package envelope

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kanengo/akasar/runtime"

	"github.com/kanengo/akasar/internal/proto"

	"github.com/kanengo/akasar/internal/pipe"

	"github.com/kanengo/akasar/runtime/protos"
)

// Child manages the child of an envelope. This is typically a child process, but
// tests may manage some in-process resources.
type Child interface {
	// Start starts the child
	Start(context.Context, *protos.AppConfig, *protos.AkasaletArgs) error

	// Wait for the child to exit
	Wait() error

	// Stdout Different IO streams connection us to the child
	Stdout() io.ReadCloser
	Stderr() io.ReadCloser

	// Pid Identifier for child process, if available.
	Pid() (int, bool)
}

type ProcessChild struct {
	cmd    *pipe.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *ProcessChild) Start(ctx context.Context, config *protos.AppConfig, args *protos.AkasaletArgs) error {
	argsEnv, err := proto.ToEnv(args)
	if err != nil {
		return fmt.Errorf("encoding akasalet  args: %w", err)
	}

	cmd := pipe.CommandContext(ctx, config.Binary, config.Args...)

	// Create pipes that capture child outputs.
	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", runtime.AkasaletArgsKey, argsEnv))
	cmd.Env = append(cmd.Env, config.Env...)

	if err := cmd.Start(); err != nil {
		return err
	}

	p.cmd = cmd
	p.stdout = outPipe
	p.stderr = errPipe

	return nil
}

func (p *ProcessChild) Wait() error {
	err := p.cmd.Wait()
	p.cmd.Cleanup()
	return err
}

func (p *ProcessChild) Stdout() io.ReadCloser {
	return p.stdout
}

func (p *ProcessChild) Stderr() io.ReadCloser {
	return p.stderr
}

func (p *ProcessChild) Pid() (int, bool) {
	if p.cmd.Process == nil {
		return 0, false
	}

	return p.cmd.Process.Pid, true
}
