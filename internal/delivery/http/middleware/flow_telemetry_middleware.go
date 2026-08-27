package middleware

import (
	"time"

	"golang-clean-architecture/internal/telemetry"

	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

// FlowTelemetry wraps a user-management business-flow entry point (register/login/update/
// logout/current) with a root span propagated via the request context, an entry counter, a
// terminal-outcome counter, and duration histograms measuring the flow's E2E business SLIs.
// It preserves the handler's return value/status exactly.
func FlowTelemetry(flow string) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		spanCtx, span := telemetry.Tracer.Start(ctx.UserContext(), flow+".flow")
		defer span.End()
		ctx.SetUserContext(spanCtx)

		start := time.Now()
		telemetry.FlowEntries.Add(spanCtx, 1, metric.WithAttributes(attribute.String("flow", flow)))

		handlerErr := ctx.Next()

		duration := time.Since(start).Seconds()
		outcome := "success"
		if handlerErr != nil {
			outcome = "error"
			span.RecordError(handlerErr)
			span.SetStatus(codes.Error, "")
		} else if ctx.Response().StatusCode() >= 400 {
			outcome = "error"
		}

		telemetry.FlowOutcomes.Add(spanCtx, 1, metric.WithAttributes(
			attribute.String("flow", flow),
			attribute.String("outcome", outcome),
		))
		telemetry.FlowDuration.Record(spanCtx, duration, metric.WithAttributes(attribute.String("flow", flow)))
		telemetry.FlowEntryToTerminalDuration.Record(spanCtx, duration, metric.WithAttributes(attribute.String("flow", flow)))

		return handlerErr
	}
}
