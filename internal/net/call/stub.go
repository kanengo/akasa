package call

import (
	"context"
	"github.com/kanengo/akasar/runtime/codegen"
	"github.com/kanengo/akasar/runtime/pool"
)

type stub struct {
	conn          Connection
	methods       []stubMethod
	injectRetries int
}

func (s *stub) Invoke(ctx context.Context, method int, args []byte, shardKey uint64) (result []byte, err error) {
	m := s.methods[method]
	opts := CallOptions{
		Retry:    m.retry,
		ShardKey: shardKey,
	}

	n := 1
	if m.retry {
		n += s.injectRetries // fake retries for testing
	}

	defer func() {
		_ = pool.FreePowerOfTwoSizeBytes(args)
	}()

	for i := 0; i < n; i++ {
		result, err = s.conn.Call(ctx, m.key, args, opts)
	}

	return

}

type stubMethod struct {
	key   MethodKey
	retry bool
}

var _ codegen.Stub = &stub{}

func NewStub(name string, reg *codegen.Registration, conn Connection, injectRetries int) codegen.Stub {
	return &stub{
		conn:          conn,
		methods:       makeStubMethods(name, reg),
		injectRetries: injectRetries,
	}
}

func makeStubMethods(fullName string, reg *codegen.Registration) []stubMethod {
	n := reg.Iface.NumMethod()
	methods := make([]stubMethod, n)
	for i := 0; i < n; i++ {
		mName := reg.Iface.Method(i).Name
		methods[i].key = MakeMethodKey(fullName, mName)
		methods[i].retry = true
	}

	for _, m := range reg.NoRetry {
		methods[m].retry = true
	}

	return methods
}
