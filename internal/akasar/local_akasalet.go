package akasar

import (
	"reflect"
	"sync"
)

type LocalAkasaLet struct {
	mu sync.Mutex
}

func (l *LocalAkasaLet) GetIface(t reflect.Type) (any, error) {
	//TODO implement me
	panic("implement me")
}

func (l *LocalAkasaLet) GetImpl(t reflect.Type) (any, error) {
	//TODO implement me
	panic("implement me")
}
