// Code generated by "akasar generate". DO NOT EDIT.
//go:build !ignoreAkasarGen

package main

import (
	"context"
	"errors"
	"github.com/kanengo/akasar"
	"github.com/kanengo/akasar/runtime/codegen"
	"github.com/kanengo/akasar/runtime/pool"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"reflect"
)

func init() {
	codegen.Register(codegen.Registration{
		Name:  "github.com/kanengo/akasar/examples/reverser/Reverser",
		Iface: reflect.TypeOf((*Reverser)(nil)).Elem(),
		Impl:  reflect.TypeOf(reverser{}),
		LocalStubFn: func(impl any, caller string, tracer trace.Tracer) any {
			return reverserLocalStub{impl: impl.(Reverser), tracer: tracer, reverseMetrics: codegen.MethodMetricsFor(codegen.MethodLabels{Caller: caller, Component: "github.com/kanengo/akasar/examples/reverser/Reverser", Method: "Reverse", Remote: false})}
		},
		ClientStubFn: func(stub codegen.Stub, caller string, tracer trace.Tracer) any {
			return reverserClientStub{stub: stub, reverseMetrics: codegen.MethodMetricsFor(codegen.MethodLabels{Caller: caller, Component: "github.com/kanengo/akasar/examples/reverser/Reverser", Method: "Reverse", Remote: true})}
		},
		ServerStubFn: func(impl any) codegen.Server {
			return reverserServerStub{impl: impl.(Reverser)}
		},
	})
	codegen.Register(codegen.Registration{
		Name:      "github.com/kanengo/akasar/Root",
		Iface:     reflect.TypeOf((*akasar.Root)(nil)).Elem(),
		Impl:      reflect.TypeOf(app{}),
		Listeners: []string{"reverser"},
		LocalStubFn: func(impl any, caller string, tracer trace.Tracer) any {
			return rootLocalStub{impl: impl.(akasar.Root), tracer: tracer}
		},
		ClientStubFn: func(stub codegen.Stub, caller string, tracer trace.Tracer) any {
			return rootClientStub{stub: stub}
		},
		ServerStubFn: func(impl any) codegen.Server {
			return rootServerStub{impl: impl.(akasar.Root)}
		},
	})
}

// akasar.InstanceOf checks.
var _ akasar.InstanceOf[Reverser] = (*reverser)(nil)
var _ akasar.InstanceOf[akasar.Root] = (*app)(nil)

// Local stub implementations.

type reverserLocalStub struct {
	impl           Reverser
	tracer         trace.Tracer
	reverseMetrics *codegen.MethodMetrics
}

// Check that reverserLocalStub implements the Reverser interface
var _ Reverser = (*reverserLocalStub)(nil)

func (s reverserLocalStub) Reverse(ctx context.Context, a0 string) (r0 string, err error) {
	// Update metrics.
	begin := s.reverseMetrics.Begin()
	defer func() { s.reverseMetrics.End(begin, err != nil, 0, 0) }()
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		// Create a child span for this method.
		ctx, span = s.tracer.Start(ctx, "main.Reverser.Reverse", trace.WithSpanKind((trace.SpanKindInternal)))
		defer func() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
		}()
	}
	defer func() {
		if err == nil {
			err = codegen.CatchResultUnwrapPanic(recover())
		}
	}()

	r0, err = s.impl.Reverse(ctx, a0)
	return
}

type rootLocalStub struct {
	impl   akasar.Root
	tracer trace.Tracer
}

// Check that rootLocalStub implements the akasar.Root interface
var _ akasar.Root = (*rootLocalStub)(nil)

// Client stub implementations.

type reverserClientStub struct {
	stub           codegen.Stub
	tracer         trace.Tracer
	reverseMetrics *codegen.MethodMetrics
}

// Check that reverserClientStub implements the Reverser interface
var _ Reverser = (*reverserClientStub)(nil)

func (s reverserClientStub) Reverse(ctx context.Context, a0 string) (r0 string, err error) {
	// Update metrics.
	var requestBytes, replyBytes int
	begin := s.reverseMetrics.Begin()
	defer func() { s.reverseMetrics.End(begin, err != nil, requestBytes, replyBytes) }()

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		ctx, span = s.tracer.Start(ctx, "main.Reverser.Reverse", trace.WithSpanKind((trace.SpanKindInternal)))
	}

	defer func() {
		// Catch and return any panics detected during encoding/decoding/rpc
		if err == nil {
			err = codegen.CatchPanics(recover())
			if err != nil {
				err = errors.Join(akasar.RemoteCallError, err)
			}
		}

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()

	}()

	// Preallocate a buffer of the right size.
	size := 0
	size += (4 + len(a0))
	enc := codegen.NewSerializer(size)

	// Encode arguments.
	enc.String(a0)
	var shardKey uint64

	// Call the remote method.
	requestBytes = len(enc.Data())
	var results []byte
	defer func() {
		pool.FreePowerOfTwoSizeBytes(results)
	}()
	results, err = s.stub.Invoke(ctx, 0, enc.Data(), shardKey)
	replyBytes = len(results)
	if err != nil {
		err = errors.Join(akasar.RemoteCallError, err)
		return
	}

	// Decode the results.
	dec := codegen.NewDeserializer(results)
	r0 = dec.String()
	err = dec.Error()

	return
}

type rootClientStub struct {
	stub   codegen.Stub
	tracer trace.Tracer
}

// Check that rootClientStub implements the akasar.Root interface
var _ akasar.Root = (*rootClientStub)(nil)

// Server stub implementation.

type reverserServerStub struct {
	impl Reverser
}

// Check that reverserServerStub is implements the codegen.Server interface.
var _ codegen.Server = (*reverserServerStub)(nil)

// GetHandleFn implements the codegen.Server interface.
func (s reverserServerStub) GetHandleFn(method string) func(ctx context.Context, args []byte) ([]byte, error) {
	switch method {
	case "Reverse":
		return s.reverse
	default:
		return nil
	}
}

func (s *reverserServerStub) reverse(ctx context.Context, args []byte) (res []byte, err error) {
	defer func() {
		if err == nil {
			err = codegen.CatchPanics(recover())
		}
	}()

	// Encode arguments.
	dec := codegen.NewDeserializer(args)
	var a0 string
	a0 = dec.String()

	r0, appErr := s.impl.Reverse(ctx, a0)

	//Encode the results.

	// Preallocate a buffer of the right size.
	size := 0
	size += (4 + len(r0))
	enc := codegen.NewSerializer(size)
	enc.String(r0)
	enc.Error(appErr)

	return enc.Data(), nil
}

type rootServerStub struct {
	impl akasar.Root
}

// Check that rootServerStub is implements the codegen.Server interface.
var _ codegen.Server = (*rootServerStub)(nil)

// GetHandleFn implements the codegen.Server interface.
func (s rootServerStub) GetHandleFn(method string) func(ctx context.Context, args []byte) ([]byte, error) {
	switch method {
	default:
		return nil
	}
}
