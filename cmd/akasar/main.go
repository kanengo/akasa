package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kanengo/akasar/internal/codegen"
)

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
	}
}
