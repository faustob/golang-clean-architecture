package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// meter is the single package-level meter for this service's HTTP server telemetry. Every
// instrument below is created from this same meter so aggregation is consistent.
var meter = otel.Meter("golang-clean-architecture")

// httpServerDuration is the OTel semantic-convention histogram for inbound HTTP request duration,
// recorded in SECONDS as required by the spec.
var httpServerDuration, httpServerDurationErr = meter.Float64Histogram(
	"http.server.request.duration",
	metric.WithUnit("s"),
	metric.WithDescription("Duration of inbound HTTP server requests, in seconds"),
)

// httpServerRequestOutcome is a low-cardinality counter (route + outcome class) that makes
// availability/error-rate SLIs computable without scanning spans.
var httpServerRequestOutcome, httpServerRequestOutcomeErr = meter.Int64Counter(
	"http.server.request.outcome",
	metric.WithDescription("Count of inbound HTTP server requests, labeled by route and outcome class"),
)

// slowRequestBudget is the latency budget used to flag slow requests for triage, aligned with the
// service's P99 HTTP response time objective.
const slowRequestBudget = 1 * time.Second

// NewTelemetryMiddleware returns a Fiber middleware that records the standard
// http.server.request.duration histogram, the outcome counter above, and a slow-request span
// event on the current span for tail-latency triage. Register it globally with app.Use(...) so
// every route is measured uniformly.
func NewTelemetryMiddleware() fiber.Handler {
	if httpServerDurationErr != nil {
		panic(fmt.Sprintf("failed to create http.server.request.duration histogram: %v", httpServerDurationErr))
	}
	if httpServerRequestOutcomeErr != nil {
		panic(fmt.Sprintf("failed to create http.server.request.outcome counter: %v", httpServerRequestOutcomeErr))
	}

	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		duration := time.Since(start)

		// Read the matched route template AFTER routing/handling has completed; reading it
		// before ctx.Next() returns an empty pattern.
		route := ctx.Route().Path
		if route == "" {
			route = "unknown"
		}
		status := ctx.Response().StatusCode()
		scheme := ctx.Protocol()
		if scheme == "" {
			scheme = "http"
		}

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", ctx.Method()),
			attribute.String("url.scheme", scheme),
			attribute.String("http.route", route),
			attribute.Int("http.response.status_code", status),
		}

		outcome := "success"
		if err != nil {
			outcome = "error"
			attrs = append(attrs, attribute.String("error.type", fmt.Sprintf("%T", err)))
		} else if status >= 500 {
			outcome = "error"
		}

		reqCtx := ctx.UserContext()

		httpServerDuration.Record(reqCtx, duration.Seconds(), metric.WithAttributes(attrs...))

		httpServerRequestOutcome.Add(reqCtx, 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("outcome", outcome),
		))

		if duration >= slowRequestBudget {
			span := trace.SpanFromContext(reqCtx)
			span.AddEvent("slow.request", trace.WithAttributes(
				attribute.String("http.route", route),
				attribute.Float64("duration.seconds", duration.Seconds()),
				attribute.Int("http.response.status_code", status),
			))
		}

		return err
	}
}
