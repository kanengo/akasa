package config

import (
	"fmt"
	"reflect"
	"strings"
)

func ComponentConfig(v reflect.Value) any {
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		panic(fmt.Errorf("invalid non pointer to struct: %v", v))
	}

	s := v.Elem()
	t := s.Type()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.Anonymous || f.Type.PkgPath() != "github.com/kanengo/akasar" ||
			!strings.HasPrefix(f.Type.Name(), "WithConfig[") {
			continue
		}

		config := s.Field(i).Addr().MethodByName("Config")
		return config.Call(nil)[0].Interface()
	}

	return nil
}
