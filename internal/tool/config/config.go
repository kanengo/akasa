package config

import (
	"fmt"

	"github.com/kanengo/akasar/runtime"
	"github.com/kanengo/akasar/runtime/protos"
	"google.golang.org/protobuf/proto"
)

type configProtoToPointer[T any, L any] interface {
	*T
	proto.Message
	GetListeners() map[string]*L
}

func GetDeployerConfig[T, L any, _ configProtoToPointer[T, L]](key, shortKey string, app *protos.AppConfig) (*T, error) {
	config := new(T)
	if err := runtime.ParseConfigSection(key, shortKey, app.Sections, config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return config, nil

}
