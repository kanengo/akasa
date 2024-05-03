package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kanengo/akasar/internal/tool/multi"

	"github.com/kanengo/akasar/internal/tool"
	"github.com/kanengo/akasar/internal/tool/single"

	"github.com/kanengo/akasar/internal/codegen"
)

//go:generate go install

const usage = `USAGE

	akasar generate			//akasar code generator
`

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprint(os.Stderr, usage)
	}
	flag.Parse()

	if len(flag.Args()) == 0 {
		_, _ = fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	deployCommands := map[string]map[string]*tool.Command{
		"single": single.Commands,
		"multi":  multi.Commands,
	}

	switch flag.Arg(0) {
	case "generate":
		generateFlags := flag.NewFlagSet("generate", flag.ExitOnError)
		generateFlags.Usage = func() {
			_, _ = fmt.Fprintln(os.Stderr, codegen.Usage)
		}
		_ = generateFlags.Parse(flag.Args()[1:])
		if err := codegen.Generate(".", flag.Args()[1:]); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	case "single", "multi":
		os.Args = os.Args[1:]
		commands := deployCommands[flag.Arg(0)]
		tool.Run(commands)
		return
	}
}
