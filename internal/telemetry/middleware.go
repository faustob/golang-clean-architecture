package telemetry

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// slowRequestBudget is the P99 latency budget for handlers in this service.
// Requests that exceed it get a span event recorded for triage, per the
// "HTTP Response Time P99" SLI recipe.
const slowRequestBudget = 1 * time.Second

// requestOutcomeCounter counts HTTP server requests by route and outcome
// class (success/client_error/server_error) so the availability and 5xx
// error-rate SLIs are computable directly from a counter without scanning
// traces or the histogram emitted by otelfiber.
var requestOutcomeCounter metric.Int64Counter

func init() {
	var err error
	requestOutcomeCounter, err = meter.Int64Counter(
		"http.server.request.outcome",
		metric.WithDescription("Count of HTTP server requests by route and outcome class"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		panic(err)
	}
}

// RequestOutcomeMiddleware records the request-outcome counter for every
// request and, for requests exceeding the service's P99 latency budget, adds
// a slow-request span event carrying the originating exception type when the
// handler returned an error. It must be registered after Fiber's recover
// middleware (so a downstream panic is still recovered before this observes
// the outcome) and before any auth/validation middleware whose rejections
// must be counted. The route template is read from ctx.Route().Path AFTER
// ctx.Next() returns, once Fiber has resolved the matched route.
func RequestOutcomeMiddleware() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		herr := ctx.Next()

		duration := time.Since(start)
		route := ctx.Route().Path
		if route == "" {
			route = "unmatched"
		}

		status := ctx.Response().StatusCode()
		outcome := "success"
		if status >= 500 {
			outcome = "server_error"
		} else if status >= 400 {
			outcome = "client_error"
		}

		attrs := []attribute.KeyValue{
			attribute.String("http.route", route),
			attribute.String("http.request.method", ctx.Method()),
			attribute.Int("http.response.status_code", status),
			attribute.String("outcome", outcome),
		}

		span := trace.SpanFromContext(ctx.UserContext())

		if herr != nil {
			errType := fmt.Sprintf("%T", herr)
			attrs = append(attrs, attribute.String("error.type", errType))
			span.SetAttributes(attribute.String("error.type", errType))
			span.SetStatus(codes.Error, herr.Error())
		}

		requestOutcomeCounter.Add(ctx.UserContext(), 1, metric.WithAttributes(attrs...))

		if duration >= slowRequestBudget {
			span.AddEvent("slow_request_budget_exceeded", trace.WithAttributes(
				attribute.String("http.route", route),
				attribute.Float64("http.server.request.duration_seconds", duration.Seconds()),
				attribute.Float64("http.server.request.budget_seconds", slowRequestBudget.Seconds()),
			))
		}

		return herr
	}
}
