package akasar

import (
	"sync"

	"github.com/kanengo/akasar/internal/reflection"
	"github.com/kanengo/akasar/internal/register"
)

type Singleton[T any] struct {
}

var singletonMu sync.Mutex

var singletons = sync.Map{}

func getSingleton[T any](key string) *register.WriteOnce[any] {
	rt := reflection.Type[T]()

	singletonMu.Lock()
	defer singletonMu.Unlock()

	mObj, ok := singletons.Load(rt)
	if !ok {
		mObj, _ = singletons.LoadOrStore(rt, &sync.Map{})
	}

	m, _ := mObj.(*sync.Map)

	s, ok := m.Load(key)
	if !ok {
		s, _ = m.LoadOrStore(key, &register.WriteOnce[any]{})
	}

	return s.(*register.WriteOnce[any])
}

func SetSingleton[T any](key string, val T) {
	s := getSingleton[T](key)
	s.Write(val)
}

// GetSingleton
// warning: 进行单列初始化的地方必须不能在读取单列的地方后后面执行，不然会死锁，
// 因为未初始化的单列读取时会阻塞
func GetSingleton[T any](key string) T {
	s := getSingleton[T](key)
	val := s.Read()

	return val.(T)
}
