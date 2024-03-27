package test

import (
	"context"

	akasa "github.com/kanengo/akasar"
)

type Reverser interface {
	Reverse(context.Context, string) (string, error)
}

type Foo struct {
}

type reverser struct {
	akasa.Components[Reverser]
	foo *Foo
}

func (*reverser) Reverse(_ context.Context, s string) (string, error) {
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-i-1] = runes[n-i-1], runes[i]
	}

	return string(runes), nil
}
