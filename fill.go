package akasar

import (
	"fmt"
	"log/slog"
	"reflect"

	"github.com/kanengo/akasar/internal/akasar"
)

func init() {
	akasar.SetLogger = setLogger
	akasar.SetAkasarInfo = setAkasarInfo
	akasar.FillRefs = fillRefs
}

func setLogger(v any, logger *slog.Logger) error {
	x, ok := v.(interface{ setLogger(*slog.Logger) })
	if !ok {
		return fmt.Errorf("setLogger: %T does not implement akasar.Components", v)
	}

	x.setLogger(logger)

	return nil
}

func setAkasarInfo(v any, info *akasar.Info) error {
	x, ok := v.(interface{ setAkasarInfo(*akasar.Info) })
	if !ok {
		return fmt.Errorf("setAkasarInfo: %T does not implement akasar.Components", v)
	}

	x.setAkasarInfo(info)

	return nil
}

func fillRefs(impl any, get func(t reflect.Type) (any, error)) error {
	p := reflect.ValueOf(impl)
	if p.Kind() != reflect.Ptr {
		return fmt.Errorf("fillRefs: %T is not a pointer", impl)
	}
	s := p.Elem()
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("fillRefs: %T is not a struct pointer", impl)
	}

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if !f.CanAddr() {
			continue
		}

		// New 一个值在此字段指针上  Ref[T]
		np := reflect.NewAt(f.Type(), f.Addr().UnsafePointer()).Interface()
		x, ok := np.(interface{ setRef(any) })
		if !ok {
			continue
		}
		valueFiled := f.Field(0) //Ref[T].val
		component, err := get(valueFiled.Type())
		if err != nil {
			return fmt.Errorf("FillRefs: setting field %v.%s: %w", s.Type(), s.Type().Field(i).Name, err)
		}
		x.setRef(component)
	}

	return nil
}
