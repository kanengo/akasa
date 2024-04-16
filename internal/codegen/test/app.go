package test

import "github.com/kanengo/akasar"

type app struct {
	akasar.Components[akasar.Root]
	akasar.Ref[Reverser]
}
