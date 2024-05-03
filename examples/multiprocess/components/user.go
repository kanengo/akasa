package components

import (
	"context"

	"github.com/kanengo/akasar"
)

type User interface {
	Login(ctx context.Context, username string, password string) error
}

type user struct {
	akasar.Components[User]
	vip akasar.Ref[VIP]
}

func (u user) Login(ctx context.Context, username string, password string) error {
	//TODO implement me
	panic("implement me")
}
