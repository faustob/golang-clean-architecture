package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// meter is the single package-level meter for this service. Every instrument
// used by the HTTP telemetry middleware is created from this meter.
var meter = otel.Meter("golang-clean-architecture")

var (
	requestOutcomeCounter metric.Int64Counter
	tenantRequestCounter  metric.Int64Counter
)

// slowRequestThreshold is the budget used to flag slow requests with a span
// event so P99 breaches can be triaged (see golang-clean-architecture-http-latency-p99).
const slowRequestThreshold = 1 * time.Second

func init() {
	var err error

	requestOutcomeCounter, err = meter.Int64Counter(
		"http.server.request.outcome.total",
		metric.WithDescription("Count of HTTP requests by matched route and outcome class (success, client_error, error)"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		otel.Handle(err)
	}

	tenantRequestCounter, err = meter.Int64Counter(
		"http.server.request.by_tenant.total",
		metric.WithDescription("Count of HTTP requests by matched route and tenant/API key"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		otel.Handle(err)
	}
}

// InitTelemetry builds the OpenTelemetry TracerProvider and MeterProvider for
// this service, exports via OTLP/gRPC (endpoint driven by the standard
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable), and registers both as
// the process-wide global providers. The returned shutdown func must be
// invoked (e.g. via defer) before the process exits so buffered telemetry is
// flushed.
func InitTelemetry(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build otel resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build otlp trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build otlp metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(shutdownCtx context.Context) error {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return meterProvider.Shutdown(shutdownCtx)
	}

	return shutdown, nil
}

// RequestTelemetryMiddleware complements the standard http.server.request.duration
// histogram emitted by otelfiber.Middleware() with:
//   - a request-outcome counter labeled by route + outcome class (availability SLI)
//   - a per-tenant request counter labeled by route + tenant (throughput SLI, per-tenant cohort)
//   - an error.type span attribute derived from the handler error (error-rate root-cause SLI)
//   - a slow_request span event when the handler exceeds the P99 latency budget
//
// It must be registered with app.Use(...) AFTER otelfiber.Middleware() so the
// active span started by otelfiber is present on the request's user context.
func RequestTelemetryMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		handlerErr := c.Next()
		duration := time.Since(start)

		route := c.Route().Path
		if route == "" {
			route = "unknown"
		}

		status := c.Response().StatusCode()
		outcome := "success"
		switch {
		case status >= 500:
			outcome = "error"
		case status >= 400:
			outcome = "client_error"
		}

		tenant := c.Get("X-API-Key")
		if tenant == "" {
			tenant = "unknown"
		}

		requestOutcomeCounter.Add(c.UserContext(), 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("outcome", outcome),
		))

		tenantRequestCounter.Add(c.UserContext(), 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("tenant", tenant),
		))

		span := trace.SpanFromContext(c.UserContext())
		if span.IsRecording() {
			if handlerErr != nil {
				span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", handlerErr)))
			}
			if duration >= slowRequestThreshold {
				span.AddEvent("slow_request", trace.WithAttributes(
					attribute.String("http.route", route),
					attribute.Float64("duration_seconds", duration.Seconds()),
				))
			}
		}

		return handlerErr
	}
}
