package akasar

import (
	"context"
	"sync"

	"github.com/kanengo/akasar/runtime/protos"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/kanengo/akasar/internal/cond"

	"github.com/kanengo/akasar/internal/net/call"
)

type routingResolver struct {
	mu        sync.Mutex
	changed   cond.Cond       // fires when endpoints changes.
	version   *call.Version   // the current version of endpoints
	endpoints []call.Endpoint // the endpoints returned by Resolver.
}

func (rr *routingResolver) IsConstant() bool {
	return false
}

func (rr *routingResolver) Resolve(ctx context.Context, version *call.Version) ([]call.Endpoint, *call.Version, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if version == nil {
		return rr.endpoints, rr.version, nil
	}

	// 当版本一样时，等待版本变化
	for *version == *rr.version {
		if err := rr.changed.Wait(ctx); err != nil {
			return nil, nil, err
		}
	}

	return rr.endpoints, rr.version, nil
}

func (rr *routingResolver) update(endpoints []call.Endpoint) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.version = &call.Version{Opaque: gonanoid.Must()}
	rr.endpoints = endpoints
	rr.changed.Broadcast()
}

func newRoutingResolver() *routingResolver {
	r := &routingResolver{
		version: &call.Version{Opaque: call.Missing.Opaque},
	}

	r.changed.L = &r.mu

	return r
}

type routingBalancer struct {
	balancer call.Balancer // balancer to user for non-routed calls

	mu sync.RWMutex
	//TODO(leeka)  support route component
	assignment *protos.Assignment

	// 地址:连接 映射
	conns map[string]call.ReplicaConnection
}

func newRoutingBalancer() *routingBalancer {
	return &routingBalancer{
		balancer: call.RoundRobin(),
		conns:    make(map[string]call.ReplicaConnection),
	}
}

func (rb *routingBalancer) Add(c call.ReplicaConnection) {
	rb.balancer.Add(c)

	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.conns[c.Address()] = c
}

func (rb *routingBalancer) Remove(c call.ReplicaConnection) {
	rb.balancer.Remove(c)

	rb.mu.Lock()
	defer rb.mu.Unlock()
	delete(rb.conns, c.Address())
}

func (rb *routingBalancer) update(assignment *protos.Assignment) {
	if assignment == nil {
		return
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.assignment = assignment
}

func (rb *routingBalancer) Pick(options call.CallOptions) (call.ReplicaConnection, bool) {
	// TODO(leeka) 目前暂不支持组件指定i路由，后续有空再加
	//if options.ShardKey == 0 {
	return rb.balancer.Pick(options)
	//}

	//rb.mu.RLock()
	//assignment := rb.assignment

}
