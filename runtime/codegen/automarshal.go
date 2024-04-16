package codegen

import (
	"fmt"
	"reflect"
	"sync"
)

type AutoMarshal interface {
	AkasarMarshal(enc *Serializer)
	AkasarUnmarshal(enc *Deserializer)
}

var (
	typesMu  sync.Mutex
	types    map[string]reflect.Type
	typeKeys map[reflect.Type]string
)

func RegisterSerializable[T AutoMarshal]() {
	var value T

	t := reflect.TypeOf(value)

	typesMu.Lock()
	defer typesMu.Unlock()

	if types == nil {
		types = make(map[string]reflect.Type)
		typeKeys = make(map[reflect.Type]string)
	}

	key := typKey(value)

	if existing, ok := types[key]; ok {
		if existing == t {
			return
		}
		panic(fmt.Sprintf("multiples types (%v and %v) have the same type string %q", existing, t, key))
	}

	types[key] = t
	typeKeys[t] = key
}

func typKey(value any) string {
	t := reflect.TypeOf(value)

	pkg := t.PkgPath()
	if pkg == "" && t.Kind() == reflect.Pointer {
		pkg = t.Elem().PkgPath()
	}

	return fmt.Sprintf("%s(%s)", t.String(), pkg)
}

// pointerTo returns a pointer to value. If value is not addressable, pointerTo
// will make an addressable copy of value and return a pointer to the copy.
func pointerTo(value any) any {
	v := reflect.ValueOf(value)
	if !v.CanAddr() {
		v = reflect.New(v.Type()).Elem()
		v.Set(reflect.ValueOf(value))
	}

	return v.Addr().Interface()
}

// pointee returns *value if value is a pointer, nil otherwise.
func pointee(value any) any {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Pointer {
		return nil
	}

	return v.Elem().Interface()
}
