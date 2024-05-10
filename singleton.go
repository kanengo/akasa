package akasar

import (
	"github.com/kanengo/akasar/internal/reflection"
	"github.com/kanengo/akasar/internal/register"
	"reflect"
	"sync"
)

type Singleton[T any] struct {
}

var singletonMu sync.Mutex
var singletons = map[reflect.Type]*register.WriteOnce[any]{}

func getSingleton[T any]() *register.WriteOnce[any] {
	rt := reflection.Type[T]()

	singletonMu.Lock()
	defer singletonMu.Unlock()

	s, ok := singletons[rt]
	if !ok {
		s = &register.WriteOnce[any]{}
		singletons[rt] = s
	}

	return s
}

func SetSingleton[T any](val T) {
	s := getSingleton[T]()
	s.Write(val)
}

func GetSingleton[T any]() T {
	s := getSingleton[T]()
	val := s.Read()

	return val.(T)
}
