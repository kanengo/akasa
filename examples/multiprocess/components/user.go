package components

import (
	"context"
	"github.com/kanengo/akasar"
	"time"
)

type User interface {
	Login(ctx context.Context, username string, password string) (UserInfo, error)
}

type UserInfo struct {
	akasar.AutoMarshal
	Id       int64
	Name     string
	CreateAt int64
	VipInfo  *VipInfo
}

type user struct {
	akasar.Components[User]
	vip akasar.Ref[VIP]
}

func (u user) Login(ctx context.Context, username string, password string) (UserInfo, error) {
	u.Logger(ctx).Info("login", "username", username, "password", password)
	vipInfo, err := u.vip.Get().GetVipInfo(ctx, 1207300042)
	if err != nil {
		return UserInfo{}, err
	}
	return UserInfo{
		Id:       1207300042,
		Name:     "kanon lee",
		CreateAt: time.Now().UnixMilli(),
		VipInfo:  vipInfo,
	}, nil
}
