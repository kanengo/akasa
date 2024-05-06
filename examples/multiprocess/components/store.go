package components

import (
	"context"

	"github.com/kanengo/akasar"
)

type Store interface {
	BuyGoods(ctx context.Context, userId int64, goodsId int32) error
}

type store struct {
	akasar.Components[Store]
}

func (s store) BuyGoods(ctx context.Context, userId int64, goodsId int32) error {
	s.Logger(ctx).Info("buyGoods", "userId", userId, "goodsId", goodsId)
	return nil
}
