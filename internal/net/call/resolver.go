package call

import "context"

type Resolver interface {
	// IsConstant 返回一个resolver是否不变的
	IsConstant() bool

	Resolve(ctx context.Context, version *Version) ([]Endpoint, *Version, error)
}

type Version struct {
	Opaque string
}

var Missing = Version{Opaque: "__tombstone__"}

type constantResolver struct {
	endpoints []Endpoint
}

var _ Resolver = &constantResolver{}

func (c *constantResolver) IsConstant() bool {
	return true
}

func (c *constantResolver) Resolve(ctx context.Context, version *Version) ([]Endpoint, *Version, error) {
	return c.endpoints, nil, nil
}

func NewConstantResolver(endpoints ...Endpoint) Resolver {
	return &constantResolver{endpoints: endpoints}
}
