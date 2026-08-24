package config

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTelemetry builds the OpenTelemetry TracerProvider and MeterProvider for
// this process and registers them (plus the W3C TraceContext/Baggage text-map
// propagator) as the GLOBAL providers (otel.Tracer(...) / otel.Meter(...) /
// otel.GetTextMapPropagator() anywhere in the codebase resolve to them from
// this point on). Go has no bytecode agent, so each binary under cmd/ owns
// and registers its own SDK exactly once, here, at startup.
//
// Registering the propagator is required for the end-to-end flow
// correlation: the Kafka producer injects trace context into message headers
// via otel.GetTextMapPropagator().Inject, and the consumer extracts it via
// otel.GetTextMapPropagator().Extract. Without registering a real
// propagator here, the global default is a no-op and that context never
// crosses the Kafka hop.
//
// The OTLP endpoint is read from the standard OTEL_EXPORTER_OTLP_ENDPOINT
// (and related) environment variables by the exporters themselves - nothing
// is hardcoded here.
//
// The returned function flushes buffered telemetry and shuts both providers
// down; callers should defer it. If exporter construction fails, telemetry
// degrades to a no-op instead of crashing the process.
func InitTelemetry(ctx context.Context, serviceName string, log *logrus.Logger) func(context.Context) error {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.WithError(err).Warn("failed to build otel resource, falling back to default resource")
		res = resource.Default()
	}

	shutdownFns := make([]func(context.Context) error, 0, 2)

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.WithError(err).Error("failed to create otlp trace exporter, tracing will be a no-op")
	} else {
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tracerProvider)
		shutdownFns = append(shutdownFns, tracerProvider.Shutdown)
	}

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.WithError(err).Error("failed to create otlp metric exporter, metrics will be a no-op")
	} else {
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(meterProvider)
		shutdownFns = append(shutdownFns, meterProvider.Shutdown)
	}

	return func(shutdownCtx context.Context) error {
		var firstErr error
		for _, fn := range shutdownFns {
			if shutdownErr := fn(shutdownCtx); shutdownErr != nil && firstErr == nil {
				firstErr = shutdownErr
			}
		}
		return firstErr
	}
}
