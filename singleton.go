package akasar

import (
	"reflect"
	"sync"

	"github.com/kanengo/akasar/internal/reflection"
	"github.com/kanengo/akasar/internal/register"
)

type Singleton[T any] struct {
}

var singletonMu sync.Mutex
var singletons = map[reflect.Type]map[string]*register.WriteOnce[any]{}

func getSingleton[T any](key string) *register.WriteOnce[any] {
	rt := reflection.Type[T]()

	singletonMu.Lock()
	defer singletonMu.Unlock()

	m, ok := singletons[rt]
	if !ok {
		m = map[string]*register.WriteOnce[any]{}
		singletons[rt] = m
	}

	s, ok := m[key]
	if !ok {
		s = &register.WriteOnce[any]{}
		m[key] = s
	}

	return s
}

func SetSingleton[T any](key string, val T) {
	s := getSingleton[T](key)
	s.Write(val)
}

func GetSingleton[T any](key string) T {
	s := getSingleton[T](key)
	val := s.Read()

	return val.(T)
}
