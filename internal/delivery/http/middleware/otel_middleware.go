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

// meter is the single package-level meter for this service; every
// instrument below is created from it.
var meter = otel.Meter("golang-clean-architecture")

var (
	httpServerRequestDuration metric.Float64Histogram
	httpRequestOutcomeTotal   metric.Int64Counter
	httpRequestsByTenantTotal metric.Int64Counter
)

func init() {
	var err error

	httpServerRequestDuration, err = meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of inbound HTTP requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create http.server.request.duration histogram: %w", err))
	}

	httpRequestOutcomeTotal, err = meter.Int64Counter(
		"http.server.request.outcome.total",
		metric.WithDescription("Count of inbound HTTP requests by route and outcome class, used for availability/error-rate SLIs"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create http.server.request.outcome.total counter: %w", err))
	}

	httpRequestsByTenantTotal, err = meter.Int64Counter(
		"http.server.requests_by_tenant.total",
		metric.WithDescription("Count of inbound HTTP requests by tenant/API key, used for per-tenant throughput SLIs"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create http.server.requests_by_tenant.total counter: %w", err))
	}
}

// slowRequestBudget is the latency budget used to flag slow requests with a
// span event for P99 tail-latency triage.
const slowRequestBudget = 500 * time.Millisecond

// OtelMetrics is a Fiber middleware that records the standard
// http.server.request.duration histogram (OTel semantic convention, in
// seconds) plus a request-outcome counter and a per-tenant request counter,
// and annotates slow requests with a span event. Register it via app.Use
// after the tracing middleware (e.g. otelfiber.Middleware()) so the span in
// ctx.UserContext() is populated.
func OtelMetrics() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		duration := time.Since(start)

		status := ctx.Response().StatusCode()
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			} else if status < fiber.StatusInternalServerError {
				status = fiber.StatusInternalServerError
			}
		}

		route := ctx.Route().Path
		if route == "" {
			route = "unmatched"
		}

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", ctx.Method()),
			attribute.String("url.scheme", ctx.Protocol()),
			attribute.Int("http.response.status_code", status),
			attribute.String("http.route", route),
		}

		if err != nil {
			attrs = append(attrs, attribute.String("error.type", fmt.Sprintf("%T", err)))
		}

		reqCtx := ctx.UserContext()

		httpServerRequestDuration.Record(reqCtx, duration.Seconds(), metric.WithAttributes(attrs...))

		outcome := "success"
		if status >= fiber.StatusInternalServerError {
			outcome = "error"
		}
		httpRequestOutcomeTotal.Add(reqCtx, 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("outcome", outcome),
		))

		tenant := ctx.Get("X-Api-Key")
		if tenant == "" {
			tenant = "unknown"
		}
		httpRequestsByTenantTotal.Add(reqCtx, 1, metric.WithAttributes(
			attribute.String("tenant", tenant),
		))

		if duration > slowRequestBudget {
			span := trace.SpanFromContext(reqCtx)
			span.AddEvent("slow_request", trace.WithAttributes(
				attribute.String("http.route", route),
				attribute.Float64("duration_seconds", duration.Seconds()),
			))
		}

		return err
	}
}
