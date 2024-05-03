package components

import (
	"context"

	"github.com/kanengo/akasar"
)

type VIP interface {
	GetVipInfo(ctx context.Context, userId int64) error
}

type vip struct {
	akasar.Components[VIP]
}

func (v vip) GetVipInfo(ctx context.Context, userId int64) error {
	//TODO implement me
	panic("implement me")
}
