package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// meter is the single package-level meter for this service. Every instrument
// (and every meter.RegisterCallback, should one be added later) MUST be
// created from this same meter instance.
var meter = otel.Meter("golang-clean-architecture")

// HTTPServerRequestDuration is the OTel semantic-convention histogram for
// inbound HTTP request duration, recorded in SECONDS. It backs the
// availability, P95/P99 latency, 5xx error-rate and request-throughput SLIs.
var HTTPServerRequestDuration otelmetric.Float64Histogram

func init() {
	h, err := meter.Float64Histogram(
		"http.server.request.duration",
		otelmetric.WithDescription("Duration of inbound HTTP server requests"),
		otelmetric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("telemetry: failed to create http.server.request.duration histogram: %v", err))
	}
	HTTPServerRequestDuration = h
}

// Setup builds the OpenTelemetry TracerProvider and MeterProvider for this
// service, exports via OTLP/gRPC, and registers both as the GLOBAL
// providers. The OTLP endpoint is env-driven via OTEL_EXPORTER_OTLP_ENDPOINT.
// The returned shutdown func must be deferred by main() so buffered spans
// and metrics flush on exit.
func Setup(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to build resource: %w", err)
	}

	var traceOpts []otlptracegrpc.Option
	var metricOpts []otlpmetricgrpc.Option
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		traceOpts = append(traceOpts, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
		metricOpts = append(metricOpts, otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithInsecure())
	}

	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create otlp trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create otlp metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(shutdownCtx context.Context) error {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("telemetry: failed to shutdown tracer provider: %w", err)
		}
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("telemetry: failed to shutdown meter provider: %w", err)
		}
		return nil
	}

	return shutdown, nil
}
