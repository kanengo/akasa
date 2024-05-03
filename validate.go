package akasar

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"unicode"

	"github.com/kanengo/akasar/internal/reflection"

	"github.com/kanengo/akasar/runtime/codegen"
)

func validateRegistrations(regs []*codegen.Registration) error {
	intfs := map[reflect.Type]struct{}{}
	for _, reg := range regs {
		intfs[reg.Iface] = struct{}{}
	}

	var errs []error
	for _, reg := range regs {
		for i := 0; i < reg.Impl.NumField(); i++ {
			f := reg.Impl.Field(i)
			switch {
			case f.Type.Implements(reflection.Type[interface{ isRef() }]()): //Ref[T]
				v := f.Type.Field(0) //Ref[T]->T
				if _, ok := intfs[v.Type]; !ok {
					err := fmt.Errorf(
						"component implementation struct %v has component reference field %v, but component %v was not registered; maybe you forgot to run 'akasar generate'",
						reg.Impl, f.Type, v.Type,
					)
					errs = append(errs, err)
				}
			case f.Type == reflection.Type[Listener]():
				name := f.Name
				if tag, ok := f.Tag.Lookup("akasar"); ok {
					if !isValidListenerName(tag) {
						err := fmt.Errorf("component implementation struct %v has invalid listener tag %q", reg.Impl, tag)
						errs = append(errs, err)
						continue
					}
					name = tag
				}
				if !slices.Contains(reg.Listeners, name) {
					err := fmt.Errorf("component implementation struct %v has a listener field %v, but listener %v hasn't been registered; maybe you forgot to run 'weaver generate'", reg.Impl, name, name)
					errs = append(errs, err)
				}
			}
		}
	}

	return errors.Join(errs...)
}

func isValidListenerName(name string) bool {
	if name == "" {
		return false
	}

	for i, c := range name {
		if !unicode.IsLetter(c) && c != '_' && (i == 0 || !unicode.IsDigit(c)) {
			return false
		}
	}

	return true
}
