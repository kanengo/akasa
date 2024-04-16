package codegen

import (
	"fmt"
	"reflect"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var globalRegistry registry

type registry struct {
	m                sync.Mutex
	components       map[reflect.Type]*Registration // by component's interface types.
	componentsByName map[string]*Registration       //by component's full name
}

type Registration struct {
	Name      string // full package-prefixed component name
	Iface     reflect.Type
	Impl      reflect.Type
	Routed    bool // True if calls to this component should be routed
	Listeners []string
	NoRetry   []int //indices of methods that should not be retried.

	LocalStubFn  func(impl any, caller string, tracer trace.Tracer) any
	ClientStubFn func(stub Stub, caller string, tracer trace.Tracer) any
	ServerStubFn func(impl any) Server
}

func Register(reg Registration) {
	if err := globalRegistry.register(reg); err != nil {
		panic(err)
	}
}

func (r *registry) register(reg Registration) error {
	r.m.Lock()
	defer r.m.Unlock()

	if old, ok := r.components[reg.Iface]; ok {
		return fmt.Errorf("component %s already registered for type %v when registering %v",
			reg.Name, old.Impl, reg.Impl)
	}

	if r.components == nil {
		r.components = map[reflect.Type]*Registration{}
	}

	if r.componentsByName == nil {
		r.componentsByName = make(map[string]*Registration)
	}

	ptr := &reg

	r.components[reg.Iface] = ptr
	r.componentsByName[reg.Name] = ptr

	return nil
}
