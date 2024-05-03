package control

import (
	"context"

	"github.com/kanengo/akasar/runtime/protos"
)

const DeployerPath = "github.com/kanengo/akasar/deployerControl"

type DeployerControl interface {
	// LogBatch 批量记录日志
	LogBatch(context.Context, *protos.LogEntryBatch) error

	// HandlerTraceSpans 处理链路追踪spans
	HandlerTraceSpans(context.Context, *protos.TraceSpans) error

	// ActivateComponent 激活组件
	// 一次ActivateComponent的调用会立即通知对应的akasalet组件的路由信息
	ActivateComponent(context.Context, *protos.ActivateComponentRequest) (*protos.ActivateComponentReply, error)

	// GetListenerAddress 返回akasalet应该监听的地址
	GetListenerAddress(context.Context, *protos.GetListenerAddressRequest) (*protos.GetListenerAddressReply, error)

	// ExportListener 导出 akasalet 的 listener, deployer会代理这些listener
	ExportListener(context.Context, *protos.ExportListenerRequest) (*protos.ExportListenerReply, error)
}
