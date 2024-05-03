package runtime

import (
	"fmt"

	"github.com/kanengo/akasar/runtime/protos"
)

const Root = "github.com/kanengo/akasar/Root"

func CheckAkasaletArgs(args *protos.AkasaletArgs) error {
	if args == nil {
		return fmt.Errorf("akasalet args is nil")
	}

	if args.App == "" {
		return fmt.Errorf("akasalet app is nil")
	}

	if args.DeploymentId == "" {
		return fmt.Errorf("akasalet deploymentId is nil")
	}

	if args.Id == "" {
		return fmt.Errorf("akasalet id is nil")
	}

	if args.ControlSocket == "" {
		return fmt.Errorf("akasalet controlSocket is nil")
	}

	return nil
}
