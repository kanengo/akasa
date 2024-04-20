package config_test

import (
	"reflect"
	"testing"

	"github.com/kanengo/akasar/internal/config"

	"github.com/kanengo/akasar"
)

func TestConfigOnTypeWithConfig(t *testing.T) {
	type testconfig struct{}
	type withConfig struct {
		akasar.WithConfig[testconfig]
	}

	got := config.ComponentConfig(reflect.ValueOf(&withConfig{}))
	if got == nil {
		t.Fatalf("unexpected nil config")
	}
	if _, ok := got.(*testconfig); !ok {
		t.Fatalf("bad config type: got %T, want *testconfig", got)
	}
}

func TestConfigOnTypeWithoutConfig(t *testing.T) {
	type withoutConfig struct{}
	if got := config.ComponentConfig(reflect.ValueOf(&withoutConfig{})); got != nil {
		t.Fatalf("unexpected non-nil config: %v", got)
	}
}
