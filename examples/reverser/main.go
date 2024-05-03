package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/kanengo/akasar"
)

type app struct {
	akasar.Components[akasar.Root]
	reverser akasar.Ref[Reverser]
	lis      akasar.Listener `akasar:"reverser"`
}

func main() {
	flag.Parse()

	if err := akasar.Run(context.Background(), func(ctx context.Context, app *app) error {
		s := "nonak eel"

		s, err := app.reverser.Get().Reverse(ctx, s)
		if err != nil {
			return err
		}

		fmt.Println(app.lis.Addr().String())

		return nil
	}); err != nil {
		fmt.Println(err)
	}
}
