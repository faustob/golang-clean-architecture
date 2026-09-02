package config

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// NewOpenTelemetry builds the TracerProvider and MeterProvider for this service and registers them
// as the GLOBAL OTel providers (otel.SetTracerProvider / otel.SetMeterProvider). Go has no bytecode
// agent, so the application is always the one responsible for managing the SDK; this must be called
// exactly once, from main(), before any instrumented code runs.
//
// Exporters are OTLP/gRPC, configured via the standard OTEL_EXPORTER_OTLP_ENDPOINT environment
// variable (defaulting to localhost:4317 per the OTel spec when unset).
//
// The returned function must be deferred by the caller to flush buffered spans/metrics on shutdown.
func NewOpenTelemetry(config *viper.Viper, log *logrus.Logger) func(ctx context.Context) {
	ctx := context.Background()

	serviceName := config.GetString("app.name")
	if serviceName == "" {
		serviceName = "golang-clean-architecture"
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		log.WithError(err).Warn("failed to merge otel resource attributes, continuing with default resource")
		res = resource.Default()
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.WithError(err).Fatal("failed to create otlp trace exporter")
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.WithError(err).Fatal("failed to create otlp metric exporter")
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return func(shutdownCtx context.Context) {
		timeoutCtx, cancel := context.WithTimeout(shutdownCtx, 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(timeoutCtx); err != nil {
			log.WithError(err).Warn("failed to shutdown otel tracer provider")
		}
		if err := mp.Shutdown(timeoutCtx); err != nil {
			log.WithError(err).Warn("failed to shutdown otel meter provider")
		}
	}
}
