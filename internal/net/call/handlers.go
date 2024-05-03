package call

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/kanengo/akasar/runtime/codegen"

	"github.com/kanengo/akasar/internal/unsafex"
)

type MethodKey [16]byte

func MakeMethodKey(component, method string) MethodKey {
	sig := sha256.Sum256(unsafex.StringToBytes(component + "." + method))
	var mk MethodKey
	copy(mk[:], sig[:])

	return mk
}

type Handler func(context.Context, []byte) ([]byte, error)

type HandlerMap struct {
	handlers map[MethodKey]Handler
	names    map[MethodKey]string
}

func NewHandlerMap() *HandlerMap {
	hm := &HandlerMap{
		handlers: make(map[MethodKey]Handler),
		names:    make(map[MethodKey]string),
	}

	// 客户端会持续调用这个handler,直到返回成功，确保服务已经ready.
	hm.Set("", "ready", func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	})

	return hm
}

func (hm *HandlerMap) Set(component, method string, handler Handler) {
	mk := MakeMethodKey(component, method)
	hm.handlers[mk] = handler
	hm.names[mk] = component + "." + method
}

func (hm *HandlerMap) AddHandlers(component string, impl any) error {
	reg, ok := codegen.Find(component)
	if !ok {
		return fmt.Errorf("component %s not found", component)
	}
	serverStub := reg.ServerStubFn(impl)
	for i, n := 0, reg.Iface.NumMethod(); i < n; i++ {
		mName := reg.Iface.Method(i).Name
		handler := serverStub.GetHandleFn(mName)
		hm.Set(component, mName, handler)
	}

	return nil
}
