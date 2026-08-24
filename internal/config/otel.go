package config

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewOtelMeterProvider builds an OTLP-exporting MeterProvider, registers it as the
// GLOBAL OpenTelemetry meter provider (otel.SetMeterProvider), and returns a shutdown
// function that must be called (e.g. via defer) so buffered metrics are flushed on exit.
//
// This is what makes the otelsql instrumentation registered in NewDatabase (see gorm.go)
// actually export telemetry instead of binding to a no-op provider: otelsql obtains its
// meter from the global provider, so the provider must be registered before NewDatabase
// runs.
//
// The OTLP endpoint is read from the standard OTEL_EXPORTER_OTLP_ENDPOINT (and related)
// environment variables by otlpmetricgrpc.New; nothing is hardcoded here.
//
// Unlike some other language SDKs, the Go otel API does not panic when a MeterProvider is
// registered more than once - otel.SetMeterProvider simply overwrites the previous global -
// so this is safe to call exactly once at process startup even in the presence of any other
// component that might also touch the global provider.
func NewOtelMeterProvider(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(otelServiceName()),
		),
	)
	if err != nil {
		return nil, err
	}

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	otel.SetMeterProvider(provider)

	return provider.Shutdown, nil
}

func otelServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return "golang-clean-architecture"
}
