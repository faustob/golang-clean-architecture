package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const serviceName = "golang-clean-architecture"

// meter is the single package-level meter for this service. Every instrument
// (and any metric.RegisterCallback) for HTTP telemetry must be created from
// this meter so callbacks stay bound to the same provider and metric names
// are never double-registered from different meters.
var meter = otel.Meter(serviceName)

// InitProviders builds the TracerProvider and MeterProvider for this
// application, registers them as the OpenTelemetry globals, and returns a
// shutdown function the caller must defer so buffered spans/metrics flush on
// exit. The OTLP/gRPC endpoint is driven by the standard
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable. Go has no bytecode agent,
// so this application (main) always owns and manages the SDK -- there is no
// pre-existing global provider to conflict with.
func InitProviders(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	return shutdown, nil
}
