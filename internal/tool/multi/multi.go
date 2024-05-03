package multi

import (
	"path/filepath"

	"github.com/kanengo/akasar/internal/tool"

	"github.com/kanengo/akasar/internal/must"

	"github.com/kanengo/akasar/runtime"
)

var (
	logDir      = filepath.Join(runtime.LogsDir(), "multi")
	dataDir     = filepath.Join(must.Must(runtime.DataDir()), "multi")
	registry    = filepath.Join(dataDir, "registry")
	traceDBFile = filepath.Join(dataDir, "traces.DB")

	Commands = map[string]*tool.Command{
		"deploy": &deployCmd,
	}
)
