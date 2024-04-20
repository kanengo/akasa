package runtime

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kanengo/akasar/internal/env"

	"github.com/BurntSushi/toml"
	"github.com/kanengo/akasar/runtime/protos"
)

func ParseConfig(file string, input string, sectionValidator func(string, string) error) (*protos.AppConfig, error) {
	var sections map[string]toml.Primitive
	_, err := toml.Decode(input, &sections)
	if err != nil {
		return nil, err
	}
	config := &protos.AppConfig{Sections: make(map[string]string)}
	for k, v := range sections {
		var buf strings.Builder
		err := toml.NewEncoder(&buf).Encode(v)
		if err != nil {
			return nil, fmt.Errorf("encoding section %q: %v", k, err)
		}
		config.Sections[k] = buf.String()
	}

	if err := extractApp(file, config); err != nil {
		return nil, err
	}

	for k, v := range config.Sections {
		if err := sectionValidator(k, v); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func ParseConfigSection(key, shortKey string, sections map[string]string, dst any) error {
	section, ok := sections[key]
	if shortKey != "" {
		if shortKeySection, ok2 := sections[shortKey]; ok2 {
			if ok {
				return fmt.Errorf("conflicting sections %q and %q", key, shortKey)
			}
			key, section, ok = shortKey, shortKeySection, ok2
		}
	}
	if !ok {
		return nil
	}

	md, err := toml.Decode(section, dst)
	if err != nil {
		return err
	}

	if unknown := md.Undecoded(); len(unknown) > 0 {
		return fmt.Errorf("section %q has unknown keys %v", key, unknown)
	}

	if x, ok := dst.(interface{ Validate() error }); ok {
		if err := x.Validate(); err != nil {
			return fmt.Errorf("section %q is invalid: %w", key, err)
		}
	}

	return nil

}

func extractApp(file string, config *protos.AppConfig) error {
	const appKey = "github.com/kanengo/akasar"
	const shortAppKey = "serviceakasar"

	type appConfig struct {
		Name     string
		Binary   string
		Args     []string
		Env      []string
		Services []string
		Rollout  time.Duration
	}

	parsed := &appConfig{}
	if err := ParseConfigSection(appKey, shortAppKey, config.Sections, parsed); err != nil {
		return err
	}

	config.Name = parsed.Name
	config.Binary = parsed.Binary
	config.Args = parsed.Args
	config.Env = parsed.Env
	config.RolloutNanos = int64(parsed.Rollout)
	config.Services = &protos.ComponentGroup{Components: parsed.Services}

	if config.Name == "" && config.Binary != "" {
		config.Name = filepath.Base(config.Binary)
	}

	if !filepath.IsAbs(config.Binary) {
		bin, err := filepath.Abs(filepath.Join(filepath.Dir(file), config.Binary))
		if err != nil {
			return err
		}
		config.Binary = bin
	}

	if _, err := env.Parse(config.Env); err != nil {
		return err
	}

	if err := checkSameService(config); err != nil {
		return err
	}

	return nil
}

func checkSameService(c *protos.AppConfig) error {
	seen := make(map[string]struct{})
	for _, component := range c.Services.Components {
		if _, ok := seen[component]; ok {
			return fmt.Errorf("component %q is repeated", component)
		}
		seen[component] = struct{}{}
	}

	return nil
}
