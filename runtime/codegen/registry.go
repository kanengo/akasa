package codegen

import (
	"fmt"
	"reflect"
	"sync"
)

var globalRegistry registry

type registry struct {
	m                sync.Mutex
	components       map[reflect.Type]*Registration // by component's interface types.
	componentsByName map[string]*Registration       //by component's full name
}

type Registration struct {
	Name  string
	Iface reflect.Type
	Impl  reflect.Type
}

func Register(reg Registration) error {
	return globalRegistry.register(reg)
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
