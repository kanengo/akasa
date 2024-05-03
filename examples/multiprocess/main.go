package main

import (
	"context"
	"fmt"
	"time"

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
		t.Logger(ctx).Info("app start")
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:

		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}
