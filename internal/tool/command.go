package tool

import (
	"context"
	"fmt"
	"os"
)

type Command struct {
	Name        string
	Description string
	Help        string
	Fn          func(ctx context.Context, args []string) error
}

func Run(commands map[string]*Command) {
	args := os.Args[1:]

	cmd, ok := commands[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(os.Stderr, "command %q not found", args[0])
		os.Exit(1)
	}

	args = args[1:]
	ctx := context.Background()

	if err := cmd.Fn(ctx, args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "command %q failed: %v", os.Args[0], err)
		os.Exit(1)
	}
}
