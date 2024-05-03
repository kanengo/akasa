package akasar

import (
	"log/slog"
	"net"
	"reflect"
)

var (
	SetLogger     func(impl any, logger *slog.Logger) error
	SetAkasarInfo func(impl any, info *Info) error
	FillRefs      func(impl any, get func(t reflect.Type) (any, error)) error
	FillListeners func(impl any, get func(string) (net.Listener, string, error)) error
)

type Info struct {
	DeploymentID string
}
