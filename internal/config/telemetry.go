package config

import (
	"context"

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

// TelemetryShutdownFunc flushes and stops the OpenTelemetry providers registered by NewTelemetry.
type TelemetryShutdownFunc func(ctx context.Context) error

// NewTelemetry builds the OpenTelemetry TracerProvider and MeterProvider, registers them as the
// GLOBAL providers (Go has no bytecode agent, so the app always owns SDK registration), and
// returns a shutdown function that flushes buffered telemetry. The OTLP endpoint is env driven
// via the standard OTEL_EXPORTER_OTLP_ENDPOINT (and friends) understood by the OTLP gRPC exporters.
func NewTelemetry(config *viper.Viper, log *logrus.Logger) TelemetryShutdownFunc {
	ctx := context.Background()

	serviceName := config.GetString("app.name")
	if serviceName == "" {
		serviceName = "golang-clean-architecture"
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		log.Warnf("failed to build otel resource, using default: %v", err)
		res = resource.Default()
	}

	noop := func(context.Context) error { return nil }

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Warnf("failed to create otlp trace exporter, tracing disabled: %v", err)
		return noop
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.Warnf("failed to create otlp metric exporter, metrics disabled: %v", err)
		return func(shutdownCtx context.Context) error {
			return tracerProvider.Shutdown(shutdownCtx)
		}
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	return func(shutdownCtx context.Context) error {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return meterProvider.Shutdown(shutdownCtx)
	}
}
