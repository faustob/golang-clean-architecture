package middleware

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"golang-clean-architecture/internal/telemetry"
)

// slowRequestBudget is the tail-latency budget used to flag slow requests
// with a span event, to help triage sustained P99 breaches.
const slowRequestBudget = 1 * time.Second

// Telemetry records the OTel semantic-convention http.server.request.duration
// histogram (seconds) for every request, with the standard http.request.method,
// url.scheme, http.route and http.response.status_code attributes. It also
// enriches the current span (created by the otelfiber middleware registered
// before this one) with an error.type attribute on failures/5xx responses,
// and adds a slow-request span event when the tail-latency budget is exceeded.
// Register this AFTER otelfiber.Middleware() so the span is already present
// in the request context.
func Telemetry() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		elapsed := time.Since(start)

		route := ctx.Route().Path
		if route == "" {
			route = "unknown"
		}

		status := ctx.Response().StatusCode()

		attrs := []attribute.KeyValue{
			semconv.HTTPRequestMethodKey.String(ctx.Method()),
			semconv.URLSchemeKey.String(ctx.Protocol()),
			semconv.HTTPRouteKey.String(route),
			semconv.HTTPResponseStatusCodeKey.Int(status),
		}

		span := trace.SpanFromContext(ctx.UserContext())

		if err != nil {
			errType := "internal_error"
			if fiberErr, ok := err.(*fiber.Error); ok {
				errType = strconv.Itoa(fiberErr.Code)
			}
			attrs = append(attrs, semconv.ErrorTypeKey.String(errType))
			if span.IsRecording() {
				span.SetAttributes(semconv.ErrorTypeKey.String(errType))
				span.SetStatus(codes.Error, err.Error())
			}
		} else if status >= 500 {
			errType := strconv.Itoa(status)
			attrs = append(attrs, semconv.ErrorTypeKey.String(errType))
			if span.IsRecording() {
				span.SetAttributes(semconv.ErrorTypeKey.String(errType))
			}
		}

		telemetry.HTTPServerRequestDuration.Record(ctx.UserContext(), elapsed.Seconds(), otelmetric.WithAttributes(attrs...))

		if span.IsRecording() && elapsed > slowRequestBudget {
			span.AddEvent("slow_request", trace.WithAttributes(
				semconv.HTTPRouteKey.String(route),
				attribute.Float64("http.server.request.duration", elapsed.Seconds()),
			))
		}

		return err
	}
}
