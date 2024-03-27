package etcd

import (
	"context"
	"time"

	"github.com/kanengo/akasar/internal/endpoint"

	"github.com/kanengo/akasar/internal/unsafex"

	clientv3 "go.etcd.io/etcd/client/v3"

	akasar "github.com/kanengo/akasar"
)

const (
	etcdScheme = "etcd"
)

type Config struct {
}

type Builder struct {
}

// New implement injector
func (b *Builder) New(ctx context.Context) (akasar.Resolver, error) {
	return nil, nil
}

type resolver struct {
	ctx context.Context
	cli *clientv3.Client
}

func (r *resolver) Resolve(ctx context.Context, target string) ([]akasar.Endpoint, error) {
	ctx, cancel := context.WithTimeout(r.ctx, time.Second*3)
	getResponse, err := r.cli.Get(ctx, target, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return nil, err
	}

	endpoints := make([]akasar.Endpoint, 0, len(getResponse.Kvs))

	for _, kv := range getResponse.Kvs {
		address := unsafex.BytesToString(kv.Key)
		ep, err := endpoint.NewDefaultEndpoint(ctx, address)
		if err != nil {
			continue
		}

		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

func (r *resolver) Scheme() string {
	return etcdScheme
}
