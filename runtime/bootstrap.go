package runtime

import (
	"context"
	"fmt"
	"os"

	"github.com/kanengo/akasar/internal/proto"

	"github.com/kanengo/akasar/runtime/protos"
)

const (
	// AkasaletArgsKey 环境变量的key
	AkasaletArgsKey = "AKASALET_ARGS"
)

type Bootstrap struct {
	Args *protos.AkasaletArgs
}

func GetBootstrap(ctx context.Context) (Bootstrap, error) {
	argsEnv := os.Getenv(AkasaletArgsKey)
	if argsEnv == "" {
		return Bootstrap{}, nil
	}

	args := &protos.AkasaletArgs{}
	if err := proto.FromEnv(argsEnv, args); err != nil {
		return Bootstrap{}, fmt.Errorf("decoding akasalet args: %w", err)
	}

	return Bootstrap{Args: args}, nil
}

func (b Bootstrap) Exists() bool {
	return b.Args != nil
}
