package test

import (
	"context"

	"github.com/kanengo/akasar"
)

type ID int64

type Reverser interface {
	Reverse(context.Context, string, *arg1, ID) (string, ID, res1, error)
	ZNoRetryMethod(ctx context.Context) error
}

var _ akasar.NotRetriable = Reverser.ZNoRetryMethod

type Foo struct {
}

type reverser struct {
	akasar.Components[Reverser]
	foo *Foo
}

type arg1 struct {
	Id     int64
	Name   string
	Skills []int32
}

type res1 struct {
	Id ID
}

func (r *reverser) ZNoRetryMethod(ctx context.Context) error {
	return nil
}

func (*reverser) Reverse(_ context.Context, s string, arg1 *arg1, id ID) (string, ID, res1, error) {
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-i-1] = runes[n-i-1], runes[i]
	}

	return string(runes), 0, res1{}, nil
}
