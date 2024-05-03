package control

import (
	"context"

	"github.com/kanengo/akasar/runtime/protos"
)

const (
	AkasaletPath = "github.com/kanengo/akasar/akasaletControl"
)

type AkasaletControl interface {
	// InitAkasalet 初始化akasalet
	InitAkasalet(context.Context, *protos.InitAkasaletRequest) (*protos.InitAkasaletReply, error)

	// UpdateComponents 更新 akasalet 应该运行的最新组件set
	UpdateComponents(context.Context, *protos.UpdateComponentsRequest) (*protos.UpdateComponentsReply, error)

	// UpdateRoutingInfo 为akasalet更新一个组件的最新路由信息
	UpdateRoutingInfo(context.Context, *protos.UpdateRoutingInfoRequest) (*protos.UpdateRoutingInfoReply, error)

	// GetHealth 获取健康信息
	GetHealth(context.Context, *protos.GetHealthRequest) (*protos.GetHealthReply, error)

	// GetLoad 获取 akasalet 负荷信息
	GetLoad(context.Context, *protos.GetLoadRequest) (*protos.GetLoadReply, error)

	// GetMetrics 获取 akasalet 指标信息
	GetMetrics(context.Context, *protos.GetMetricsRequest) (*protos.GetMetricsReply, error)

	// GetProfile 获取 akasalet profile
	GetProfile(context.Context, *protos.GetProfileRequest) (*protos.GetProfileReply, error)
}
