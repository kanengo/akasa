package akasar

import "context"

type Resolver interface {
	Resolve(ctx context.Context, target string) ([]Endpoint, error)
	Scheme() string
}

type Endpoint interface {
	Call(ctx context.Context, data []byte) ([]byte, error)
	Close() error
	Name() string
}
