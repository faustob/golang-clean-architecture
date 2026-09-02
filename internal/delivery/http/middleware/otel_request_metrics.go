package middleware

import (
	"fmt"
	"time"

	"sync"

	"golang-clean-architecture/internal/telemetry"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	requestDurationHistogram metric.Float64Histogram
	requestOutcomeCounter    metric.Int64Counter
	tenantRequestCounter     metric.Int64Counter
	instrumentsOnce          sync.Once
	instrumentsErr           error
)

// ensureInstruments lazily creates the instruments on first use, after
// InitOTel has populated telemetry.Meter (or, if InitOTel has not yet run,
// against otel.Meter's delegating no-op meter which is safe to call).
func ensureInstruments() error {
	instrumentsOnce.Do(func() {
		meter := telemetry.Meter
		if meter == nil {
			meter = otel.Meter("golang-clean-architecture")
		}

		var err error

		requestDurationHistogram, err = meter.Float64Histogram(
			"http.server.request.duration",
			metric.WithDescription("Duration of inbound HTTP requests"),
			metric.WithUnit("s"),
		)
		if err != nil {
			instrumentsErr = err
			return
		}

		requestOutcomeCounter, err = meter.Int64Counter(
			"http.server.request.outcomes",
			metric.WithDescription("Count of HTTP requests by route and outcome class, for availability/error-rate SLIs"),
		)
		if err != nil {
			instrumentsErr = err
			return
		}

		tenantRequestCounter, err = meter.Int64Counter(
			"http.server.requests.by_tenant",
			metric.WithDescription("Count of HTTP requests broken out by tenant/API key, for per-tenant throughput SLIs"),
		)
		if err != nil {
			instrumentsErr = err
			return
		}
	})
	return instrumentsErr
}

// RequestOutcomeMiddleware records the standard http.server.request.duration
// histogram plus request-outcome, per-tenant throughput, and slow-request
// telemetry for every request handled by the Fiber app. It must run after
// middleware.Recoverer (if present) and before route-level auth/validation
// middleware so rejected requests are still observed.
func RequestOutcomeMiddleware() fiber.Handler {
	const slowRequestThreshold = 500 * time.Millisecond

	return func(ctx *fiber.Ctx) error {
		if err := ensureInstruments(); err != nil {
			return err
		}

		start := time.Now()

		err := ctx.Next()

		duration := time.Since(start)
		status := ctx.Response().StatusCode()

		var errType string
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			} else if status < 400 {
				status = fiber.StatusInternalServerError
			}
			errType = fmt.Sprintf("%T", err)
		}

		route := ctx.Route().Path
		scheme := "http"
		if ctx.Protocol() == "https" {
			scheme = "https"
		}

		attrs := []attribute.KeyValue{
			attribute.String("http.request.method", ctx.Method()),
			attribute.String("url.scheme", scheme),
			attribute.Int("http.response.status_code", status),
			attribute.String("http.route", route),
		}
		if errType != "" {
			attrs = append(attrs, attribute.String("error.type", errType))
		}
		requestDurationHistogram.Record(ctx.UserContext(), duration.Seconds(), metric.WithAttributes(attrs...))

		outcome := "success"
		if status >= 500 {
			outcome = "server_error"
		} else if status >= 400 {
			outcome = "client_error"
		}
		requestOutcomeCounter.Add(ctx.UserContext(), 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("outcome", outcome),
		))

		tenant := ctx.Get("X-API-Key")
		if tenant == "" {
			tenant = "unknown"
		}
		tenantRequestCounter.Add(ctx.UserContext(), 1, metric.WithAttributes(
			attribute.String("tenant", tenant),
			attribute.String("http.route", route),
		))

		if duration > slowRequestThreshold {
			span := trace.SpanFromContext(ctx.UserContext())
			span.AddEvent("slow_request", trace.WithAttributes(
				attribute.String("http.route", route),
				attribute.Float64("duration_seconds", duration.Seconds()),
			))
		}

		if errType != "" {
			span := trace.SpanFromContext(ctx.UserContext())
			span.SetAttributes(attribute.String("error.type", errType))
		}

		return err
	}
}
