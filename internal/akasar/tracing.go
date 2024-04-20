package akasar

import (
	"os"

	"go.opentelemetry.io/otel/propagation"

	"go.opentelemetry.io/otel"

	"github.com/kanengo/akasar/internal/traceio"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

func tracer(exporter sdktrace.SpanExporter, app, deploymentId string, akasaletId string) trace.Tracer {
	const instrumentationLibrary = "github.com/kanengo/akasar/serviceakasar"
	const instrumentationVersion = "0.0.1"

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(app),
			semconv.ServiceInstanceID(akasaletId),
			semconv.ProcessPID(os.Getpid()),
			traceio.App(app),
			traceio.DeploymentId(deploymentId),
			traceio.AkasaletId(akasaletId),
		)),
		// TODO(leeka): Allow the user to create new TracerProviders where
		// they can control trace sampling and other options.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	tracer := tracerProvider.Tracer(instrumentationLibrary, trace.WithInstrumentationVersion(instrumentationVersion))

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tracer

}
