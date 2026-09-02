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

// meter is the single package-level meter for this service's HTTP layer.
// Every HTTP instrument (and any future observable callback) must be
// created from this same meter so callbacks and instruments stay wired
// together.
var meter = otel.Meter("golang-clean-architecture")

// HTTPInstruments holds the request-level metric instruments recorded by the
// Telemetry middleware.
type HTTPInstruments struct {
	RequestDuration metric.Float64Histogram
	RequestOutcome  metric.Int64Counter
}

// NewHTTPInstruments creates the HTTP server instruments backing the
// availability, latency (p95/p99), error-rate and throughput SLIs.
func NewHTTPInstruments() (*HTTPInstruments, error) {
	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.request.duration histogram: %w", err)
	}

	requestOutcome, err := meter.Int64Counter(
		"http.server.request.outcome.total",
		metric.WithUnit("1"),
		metric.WithDescription("Count of inbound HTTP requests by route and outcome class"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.request.outcome.total counter: %w", err)
	}

	return &HTTPInstruments{RequestDuration: requestDuration, RequestOutcome: requestOutcome}, nil
}

// Telemetry records the standard http.server.request.duration histogram and
// a request-outcome counter (route + outcome class, for the availability
// SLI) for every request. It also annotates the active server span (created
// by the otelfiber middleware registered ahead of it) with the matched
// route, maps the originating error to a low-cardinality error.type, and
// emits a slow-request span event when the handler exceeds the P99 latency
// budget.
func Telemetry(instruments *HTTPInstruments) fiber.Handler {
	const slowRequestBudget = 500 * time.Millisecond

	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		duration := time.Since(start)

		route := ctx.Route().Path
		if route == "" {
			route = "unknown"
		}

		status := ctx.Response().StatusCode()
		if err != nil {
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			} else if status < 400 {
				status = fiber.StatusInternalServerError
			}
		}

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
		}

		if span.IsRecording() && duration > slowRequestBudget {
			span.AddEvent("slow_request", trace.WithAttributes(
				attribute.Float64("duration.seconds", duration.Seconds()),
				attribute.String("http.route", route),
			))
		}

		instruments.RequestDuration.Record(ctx.UserContext(), duration.Seconds(), metric.WithAttributes(attrs...))

		outcomeClass := "success"
		if status >= 500 {
			outcomeClass = "server_error"
		} else if status >= 400 {
			outcomeClass = "client_error"
		}

		instruments.RequestOutcome.Add(ctx.UserContext(), 1, metric.WithAttributes(
			attribute.String("http.route", route),
			attribute.String("outcome", outcomeClass),
		))

		return err
	}
}
