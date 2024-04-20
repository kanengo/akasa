package local

import (
	"path/filepath"

	"github.com/kanengo/akasar/internal/must"

	"github.com/kanengo/akasar/runtime"
)

var (
	dataDir      = filepath.Join(must.Must(runtime.DataDir()), ".local")
	RegistryDir  = filepath.Join(dataDir, "registry")
	TracesDBFile = filepath.Join(dataDir, "traces.DB")
)
