package config

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewMeterProvider builds an OTLP/gRPC metric exporter and reader and
// registers the resulting MeterProvider as the GLOBAL OpenTelemetry
// MeterProvider. The exporter endpoint is env-driven (it follows the
// OTEL_EXPORTER_OTLP_ENDPOINT / OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
// convention used by otlpmetricgrpc) rather than hardcoded.
//
// This must be called once, in main(), before any auto-instrumented code
// path (e.g. the otelsql-wrapped database connection) runs, since
// otelsql records through the GLOBAL meter provider.
func NewMeterProvider(log *logrus.Logger) *sdkmetric.MeterProvider {
	ctx := context.Background()

	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.Fatalf("failed to create otlp metric exporter: %v", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("golang-clean-architecture"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create otel resource: %v", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)

	// Registered once, defensively, at startup only. Go's otel.SetMeterProvider
	// simply replaces the global provider rather than panicking on a second
	// call, so this is safe even if something else already set one.
	otel.SetMeterProvider(mp)

	return mp
}
