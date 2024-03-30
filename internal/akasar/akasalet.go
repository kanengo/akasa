package akasar

import "reflect"

type AkasaLet interface {
	// GetIface 根据接口类型获取组件的handle
	GetIface(t reflect.Type) (any, error)

	// GetImpl 根据提供的类型获取组件的实现
	GetImpl(t reflect.Type) (any, error)
}
