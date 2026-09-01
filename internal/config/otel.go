package config

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewOtelSDK builds the OpenTelemetry tracer and meter providers and registers them as the
// GLOBAL providers for this process, so every package obtaining otel.Tracer(...)/otel.Meter(...)
// binds to a real, exporting SDK instead of the no-op default.
//
// The OTLP endpoint is env-driven (OTEL_EXPORTER_OTLP_ENDPOINT / OTEL_EXPORTER_OTLP_*), never
// hardcoded here. Callers MUST defer the returned shutdown func so buffered telemetry flushes.
func NewOtelSDK(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("golang-clean-architecture"),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	// Go's OTel API does not panic on re-registration (unlike Java's GlobalOpenTelemetry.set), so
	// this is safe to call even if something else in the process already registered providers --
	// it simply becomes the active global provider for this process.
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	return func(shutdownCtx context.Context) error {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return meterProvider.Shutdown(shutdownCtx)
	}, nil
}
