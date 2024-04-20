package user

import (
	"context"

	"github.com/kanengo/akasar"
)

type Login interface {
	Login(ctx context.Context, token string) (*Info, error)
}

type Info struct {
	akasar.AutoMarshal
	Id       int64
	NickName string
	Balances int64
}

type login struct {
	akasar.Components[Login]
}

func (l login) Login(ctx context.Context, token string) (*Info, error) {
	return &Info{
		Id:       1,
		NickName: "user",
		Balances: 10_000_000,
	}, nil
}
