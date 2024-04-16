package codegen

import (
	"context"
)

// Stub 允许一个 akasar 组件通过 RPC 方式调用另一个进程的组件方法
type Stub interface {
	//Tracer() trace.Tracer

	// Invoke 执行rpc调用。
	//代码生成时, 接口方法已经排序， method 就是方法切片的索引, shardKey 是路由组件的 shard key
	Invoke(ctx context.Context, method int, args []byte, shardKey uint64) (result []byte, err error)
}

// Server 允许一个进程里的一个 akasar component 可以通过 RPC 接收其他进程的组件指定执行的方法
type Server interface {
	GetHandleFn(method string) func(ctx context.Context, args []byte) ([]byte, error)
}
