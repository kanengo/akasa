package components

import (
	"context"

	"github.com/kanengo/akasar"
)

type Store interface {
	BuyGoods(ctx context.Context, req BuyGoodsRequest) error
}

type store struct {
	akasar.Components[Store]
}

var _ Store = (*store)(nil)

type BuyGoodsRequest struct {
	UserId  int64
	GoodsId int32
	BuyNum  uint32
	akasar.AutoMarshal
}

func (s store) BuyGoods(ctx context.Context, req BuyGoodsRequest) error {
	s.Logger(ctx).Info("buyGoods", "userId", req.UserId, "goodsId", req.GoodsId)
	return nil
}
