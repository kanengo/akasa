package env

import (
	"fmt"
	"strings"
)

func Split(kv string) (string, string, error) {
	k, v, ok := strings.Cut(kv, "=")
	if !ok {
		return "", "", fmt.Errorf("env:%q is not of form key=value", kv)
	}

	return k, v, nil
}

func Parse(env []string) (map[string]string, error) {
	kvs := make(map[string]string, len(env))
	for _, kv := range env {
		k, v, err := Split(kv)
		if err != nil {
			return nil, err
		}
		kvs[k] = v
	}

	return kvs, nil
}
