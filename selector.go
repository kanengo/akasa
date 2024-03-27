package akasar

import "context"

type Selector interface {
	Select(ctx context.Context) (Endpoint, DoneFunc, error)
}

type DoneInfo struct {
	Err           error
	BytesSent     bool
	BytesReceived bool
}

type DoneFunc func(ctx context.Context, di DoneInfo)
