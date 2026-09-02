package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer and Meter are the single package-level instruments used across the
// service. Every instrument and callback in this codebase must be created
// from Meter so callbacks fire against the same MeterProvider that owns
// their observable instruments.
var (
	Tracer trace.Tracer
	Meter  metric.Meter
)

// InitOTel builds the TracerProvider and MeterProvider once, registers them
// as the process-wide OpenTelemetry globals, and returns a shutdown function
// that must be deferred by the caller (main) so buffered telemetry flushes
// on exit. The OTLP gRPC endpoint is driven entirely by the standard
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable.
func InitOTel(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build otel resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	Tracer = otel.Tracer(serviceName)
	Meter = otel.Meter(serviceName)

	shutdown := func(shutdownCtx context.Context) error {
		if shutdownErr := tp.Shutdown(shutdownCtx); shutdownErr != nil {
			return shutdownErr
		}
		return mp.Shutdown(shutdownCtx)
	}

	return shutdown, nil
}
