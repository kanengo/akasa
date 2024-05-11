package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/codes"

	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel"

	"github.com/kanengo/akasar"
	"github.com/kanengo/akasar/examples/multiprocess/components"
)

type app struct {
	akasar.Components[akasar.Root]
	user  akasar.Ref[components.User]
	store akasar.Ref[components.Store]
}

func main() {
	err := akasar.Run(context.Background(), func(ctx context.Context, t *app) error {
		tracer := otel.GetTracerProvider().Tracer("multi process main")
		t.Logger(ctx).Info("app start")
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Logger(ctx).Debug("app end ctx done")
				return nil
			case <-ticker.C:
				err := func() error {
					var err error
					ctx, span := tracer.Start(ctx, "main span", trace.WithSpanKind(trace.SpanKindClient))
					defer func() {
						if err != nil {
							span.RecordError(err)
							span.SetStatus(codes.Error, err.Error())
						}
						span.End()
					}()
					user, err := t.user.Get().Login(ctx, "leeka", "123456")
					if err != nil {
						return err
					}
					t.Logger(ctx).Info("login success", "userId", user.Id, "vip", user.VipInfo.VipLevel)
					_ = t.store.Get().BuyGoods(ctx, components.BuyGoodsRequest{
						UserId:  user.Id,
						GoodsId: 10001,
						BuyNum:  1,
					})
					return nil
				}()
				if err != nil {
					return err
				}
			}
		}
	})
	if err != nil {
		fmt.Println(err)
	}
}
