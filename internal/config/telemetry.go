package config

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTelemetry builds the OpenTelemetry SDK (OTLP/gRPC exporters for traces and metrics),
// registers it as the global provider, and returns a shutdown function to flush on exit.
// The exporter endpoint is read from the standard OTEL_EXPORTER_OTLP_ENDPOINT env var (and
// related OTEL_EXPORTER_OTLP_* vars) by the exporters themselves - never hardcoded here.
//
// otel.SetTracerProvider/SetMeterProvider only swap the global delegate in the Go SDK (unlike
// some other languages, they do not error/panic if a provider was already registered, e.g. by
// an externally attached agent), so this call is safe to make unconditionally at startup.
func InitTelemetry(ctx context.Context, log *logrus.Logger) func(context.Context) error {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "user-management-service"
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		log.WithError(err).Warn("failed to build otel resource, falling back to default")
		res = resource.Default()
	}

	noop := func(context.Context) error { return nil }

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to create otlp trace exporter, tracing disabled")
		return noop
	}

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to create otlp metric exporter, metrics disabled")
		return noop
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	return func(shutdownCtx context.Context) error {
		_ = tracerProvider.Shutdown(shutdownCtx)
		_ = meterProvider.Shutdown(shutdownCtx)
		return nil
	}
}
