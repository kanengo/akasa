package akasar

import (
	"context"
	"fmt"

	"github.com/kanengo/akasar/internal/control"
	"github.com/kanengo/akasar/runtime/protos"
)

type akasaletControl control.AkasaletControl

// noopAkasaletControl control.AkasaletControl的空实现，目的是先注册好 control.AkasaletControl 组件
// 实际实现👉 internal/akasar/remote_akasalet.go
type noopAkasaletControl struct {
	Components[akasaletControl]
}

var _ akasaletControl = (*noopAkasaletControl)(nil)

func (*noopAkasaletControl) InitAkasalet(ctx context.Context, request *protos.InitAkasaletRequest) (*protos.InitAkasaletReply, error) {
	return nil, fmt.Errorf("akasaletControl.InitAkasalet not implemented")
}

func (*noopAkasaletControl) UpdateComponents(ctx context.Context, request *protos.UpdateComponentsRequest) (*protos.UpdateComponentsReply, error) {
	return nil, fmt.Errorf("akasaletControl.UpdateComponents not implemented")
}

func (*noopAkasaletControl) UpdateRoutingInfo(ctx context.Context, request *protos.UpdateRoutingInfoRequest) (*protos.UpdateRoutingInfoReply, error) {
	return nil, fmt.Errorf("akasaletControl.UpdateRoutingInfo not implemented")
}

func (*noopAkasaletControl) GetHealth(ctx context.Context, request *protos.GetHealthRequest) (*protos.GetHealthReply, error) {
	return nil, fmt.Errorf("akasaletControl.GetHealth not implemented")
}

func (*noopAkasaletControl) GetLoad(ctx context.Context, request *protos.GetLoadRequest) (*protos.GetLoadReply, error) {
	return nil, fmt.Errorf("akasaletControl.GetLoad not implemented")
}

func (*noopAkasaletControl) GetMetrics(ctx context.Context, request *protos.GetMetricsRequest) (*protos.GetMetricsReply, error) {
	return nil, fmt.Errorf("akasaletControl.GetMetrics not implemented")
}

func (*noopAkasaletControl) GetProfile(ctx context.Context, request *protos.GetProfileRequest) (*protos.GetProfileReply, error) {
	return nil, fmt.Errorf("akasaletControl.GetProfile not implemented")
}
