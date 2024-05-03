package akasar

import (
	"context"
	"fmt"
	"github.com/kanengo/akasar/internal/control"
	"github.com/kanengo/akasar/runtime/logging"
	"github.com/kanengo/akasar/runtime/protos"
	"log/slog"
	"os"
)

type deployerControl control.DeployerControl

type localDeployerControl struct {
	Components[deployerControl]
	pp *logging.PrettyPrinter
}

var _ deployerControl = (*localDeployerControl)(nil)

func (l *localDeployerControl) Init(ctx context.Context) error {
	l.pp = logging.NewPrettyPrinter()
	return nil
}

func (l *localDeployerControl) LogBatch(ctx context.Context, batch *protos.LogEntryBatch) error {
	for _, entry := range batch.Entries {
		if entry.Level == slog.LevelError.String() {
			_, _ = fmt.Fprintln(os.Stderr, l.pp.Format(entry))
		} else {
			_, _ = fmt.Fprintln(os.Stdout, l.pp.Format(entry))
		}
	}
	return nil
}

func (l *localDeployerControl) HandlerTraceSpans(ctx context.Context, spans *protos.TraceSpans) error {
	return fmt.Errorf("localDeployerControl.HandleTraceSpans not implemented")
}

func (l *localDeployerControl) ActivateComponent(ctx context.Context, request *protos.ActivateComponentRequest) (*protos.ActivateComponentReply, error) {
	return nil, fmt.Errorf("localDeployerControl.ActivateComponent not implemented")
}

func (l *localDeployerControl) GetListenerAddress(ctx context.Context, request *protos.GetListenerAddressRequest) (*protos.GetListenerAddressReply, error) {
	return nil, fmt.Errorf("localDeployerControl.GetListenerAddress not implemented")
}

func (l *localDeployerControl) ExportListener(ctx context.Context, request *protos.ExportListenerRequest) (*protos.ExportListenerReply, error) {
	return nil, fmt.Errorf("localDeployerControl.ExportListener not implemented")
}
