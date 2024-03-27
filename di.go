package akasar

import "context"

type Injector[T any] interface {
	New(context.Context) (T, error)
}
