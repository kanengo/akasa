package main

import (
	"context"
	"fmt"

	"github.com/kanengo/akasar"
)

type Reverser interface {
	Reverse(context.Context, string) (string, error)
}

type reverser struct {
	akasar.Components[Reverser]
	akasar.WithConfig[config]
}

type ecs struct {
	AppId     string
	AppSecret string
}

type config struct {
	DB    string
	Redis string
	Ecs   struct {
		AppId     string
		AppSecret string
	}
}

func (r reverser) Init(ctx context.Context) error {
	fmt.Println("cfg", r.Config())
	return nil
}

func (r reverser) Reverse(ctx context.Context, s string) (string, error) {

	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-i-1] = runes[n-i-1], runes[i]
	}

	result := akasar.NewResult(string(runes))

	res := result.Unwrap()
	r.Logger(ctx).InfoContext(ctx, "Reverser", "before", s, "after", res)

	return res, nil
}
