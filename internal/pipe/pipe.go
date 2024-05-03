package pipe

import (
	"context"
	"io"
	"os/exec"
)

type Cmd struct {
	*exec.Cmd

	closeAfterStart []io.Closer
	closeAfterWait  []io.Closer
}

func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	return &Cmd{
		Cmd:             exec.CommandContext(ctx, name, arg...),
		closeAfterStart: nil,
		closeAfterWait:  nil,
	}
}

func (c *Cmd) Start() error {
	if err := c.Cmd.Start(); err != nil {
		return err
	}
	closeAll(&c.closeAfterStart)
	return nil
}

func (c *Cmd) Wait() error {
	if err := c.Cmd.Wait(); err != nil {
		return err
	}
	closeAll(&c.closeAfterWait)
	return nil
}

func (c *Cmd) Cleanup() {
	closeAll(&c.closeAfterStart)
	closeAll(&c.closeAfterWait)
}

func closeAll(file *[]io.Closer) {
	for _, f := range *file {
		_ = f.Close()
	}
	*file = nil
}
