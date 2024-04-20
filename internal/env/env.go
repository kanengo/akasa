package env

import "fmt"

func Parse(env []string) (map[string]string, error) {
	if len(env)%2 != 0 {
		return nil, fmt.Errorf("env length must be even")
	}

	kvs := make(map[string]string, len(env)/2)
	for i := 0; i < len(env); i += 2 {
		kvs[env[i]] = env[i+1]
	}

	return kvs, nil
}
