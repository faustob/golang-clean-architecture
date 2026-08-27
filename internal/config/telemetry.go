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

// NewTelemetry builds the OpenTelemetry TracerProvider and MeterProvider for this service,
// exporting via OTLP/gRPC (endpoint configured through the standard OTEL_EXPORTER_OTLP_ENDPOINT
// env var), and registers them as the process-wide global providers. otelsql (used to open the
// MySQL connection) and any otel.Tracer/otel.Meter call in the app bind to these global
// providers, so registration must happen before the database is opened.
//
// It returns a shutdown function that must be called (e.g. via defer) to flush buffered
// telemetry before the process exits.
func NewTelemetry(viperConfig *viper.Viper, log *logrus.Logger) func(context.Context) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(viperConfig.GetString("app.name")),
		),
	)
	if err != nil {
		log.Fatalf("failed to create otel resource: %v", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatalf("failed to create otlp trace exporter: %v", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		log.Fatalf("failed to create otlp metric exporter: %v", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	return func(shutdownCtx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 5*time.Second)
		defer cancel()
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return meterProvider.Shutdown(shutdownCtx)
	}
}
