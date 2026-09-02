package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// meter is the single package-level OTel meter for this service; every instrument
// for HTTP telemetry MUST be created from this meter.
var meter = otel.Meter("golang-clean-architecture")

// httpServerRequestDuration is the standard OTel semantic-convention histogram for
// inbound HTTP request duration, recorded in seconds. It backs the availability,
// P95/P99 latency, error-rate and throughput SLIs for this service.
var httpServerRequestDuration metric.Float64Histogram

func init() {
	var err error
	httpServerRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of inbound HTTP server requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create http.server.request.duration histogram: %v", err))
	}
}

// slowRequestThreshold is the latency budget used to annotate handlers that breach
// the P99 target with a span event for triage.
const slowRequestThreshold = 1 * time.Second

// NewOtelMetricsMiddleware records the http.server.request.duration histogram for
// every request, tagged with the matched route template, method, scheme and
// response status, and annotates the active span (started by the otelfiber
// middleware) with error/slow-request details.
func NewOtelMetricsMiddleware() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		duration := time.Since(start)

		// chi/fiber route pattern is only populated after routing has happened,
		// i.e. after ctx.Next() returns.
		route := ctx.Route().Path
		if route == "" {
			route = "unmatched"
		}

		status := ctx.Response().StatusCode()

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", ctx.Method()),
			attribute.String("url.scheme", ctx.Protocol()),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		}

		span := trace.SpanFromContext(ctx.UserContext())

		if err != nil {
			errType := fmt.Sprintf("%T", err)
			attrs = append(attrs, attribute.String("error.type", errType))
			if span.IsRecording() {
				span.SetAttributes(attribute.String("error.type", errType))
				span.SetStatus(codes.Error, err.Error())
			}
		} else if status >= 500 {
			errType := fmt.Sprintf("HTTP_%d", status)
			attrs = append(attrs, attribute.String("error.type", errType))
			if span.IsRecording() {
				span.SetAttributes(attribute.String("error.type", errType))
			}
		}

		httpServerRequestDuration.Record(ctx.UserContext(), duration.Seconds(), metric.WithAttributes(attrs...))

		if duration >= slowRequestThreshold && span.IsRecording() {
			span.AddEvent("slow.request", trace.WithAttributes(
				attribute.String("http.route", route),
				attribute.Float64("duration.seconds", duration.Seconds()),
			))
		}

		return err
	}
}
