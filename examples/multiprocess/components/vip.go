package components

import (
	"context"
	"time"

	"github.com/kanengo/akasar"
)

type VIP interface {
	GetVipInfo(ctx context.Context, userId int64) (*VipInfo, error)
}

type vip struct {
	akasar.Components[VIP]
}

type VipLevelType int32

const (
	VipTypeNone  VipLevelType = iota
	VipTypeBasic              = iota
)

type VipInfo struct {
	akasar.AutoMarshal
	VipLevel VipLevelType
	ExpireAt int64
}

func (v vip) GetVipInfo(ctx context.Context, userId int64) (*VipInfo, error) {
	return &VipInfo{
		VipLevel: VipTypeBasic,
		ExpireAt: time.Now().Add(time.Hour).UnixMilli(),
	}, nil
}
