package single

import (
	"path/filepath"

	"github.com/kanengo/akasar/internal/tool"

	"github.com/kanengo/akasar/internal/must"

	"github.com/kanengo/akasar/runtime"
)

var (
	dataDir      = filepath.Join(must.Must(runtime.DataDir()), "single")
	RegistryDir  = filepath.Join(dataDir, "registry")
	TracesDBFile = filepath.Join(dataDir, "traces.DB")

	Commands = map[string]*tool.Command{
		"deploy": &deployCmd,
	}
)
