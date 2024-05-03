package call

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

const traceHeaderLen = 25

func writeTraceContext(ctx context.Context, b []byte) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	traceID := sc.TraceID()
	spanID := sc.SpanID()

	copy(b, traceID[:])
	copy(b[16:], spanID[:])
	b[24] = byte(sc.TraceFlags())
}

func readTraceContext(b []byte) trace.SpanContext {
	cfg := trace.SpanContextConfig{
		TraceID:    *(*trace.TraceID)(b[:16]),
		SpanID:     *(*trace.SpanID)(b[16:24]),
		TraceFlags: trace.TraceFlags(b[24]),
		Remote:     true,
	}

	return trace.NewSpanContext(cfg)
}
