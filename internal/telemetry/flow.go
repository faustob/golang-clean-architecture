package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Tracer and meter are the single, package-level instances for the User Management business
// flow. Every instrument below is created from this same meter (never redeclare instruments
// with the same name from a meter created elsewhere).
var (
	Tracer trace.Tracer = otel.Tracer("golang-clean-architecture/user-flow")
	meter               = otel.Meter("golang-clean-architecture/user-flow")

	// FlowEntries counts every invocation of a user-management flow entry point, independent
	// of its eventual outcome (throughput SLI).
	FlowEntries metric.Int64Counter
	// FlowOutcomes counts terminal outcomes (success/error) of the user-management flow
	// (success-rate SLI).
	FlowOutcomes metric.Int64Counter
	// FlowValidationOutcomes counts the pass/fail outcome of each per-step validation within
	// the flow (validation-failure-rate SLI).
	FlowValidationOutcomes metric.Int64Counter
	// FlowDuration records the end-to-end duration of the flow, in seconds (latency SLI).
	FlowDuration metric.Float64Histogram
	// FlowEntryToTerminalDuration records the wall-clock time between flow entry and its
	// terminal state, in seconds (freshness SLI).
	FlowEntryToTerminalDuration metric.Float64Histogram
)

func init() {
	var err error

	if FlowEntries, err = meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Count of entries into the user management business flow"),
	); err != nil {
		log.Printf("otel: failed to create flow.entries counter: %v", err)
	}

	if FlowOutcomes, err = meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Terminal outcomes (success/error) of the user management business flow"),
	); err != nil {
		log.Printf("otel: failed to create flow.outcomes counter: %v", err)
	}

	if FlowValidationOutcomes, err = meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Pass/fail outcome of each per-step validation in the user management flow"),
	); err != nil {
		log.Printf("otel: failed to create flow.validation.outcomes counter: %v", err)
	}

	if FlowDuration, err = meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of the user management business flow"),
		metric.WithUnit("s"),
	); err != nil {
		log.Printf("otel: failed to create flow.duration histogram: %v", err)
	}

	if FlowEntryToTerminalDuration, err = meter.Float64Histogram(
		"flow.entry_to_terminal.duration",
		metric.WithDescription("Wall-clock time between flow entry and its terminal state"),
		metric.WithUnit("s"),
	); err != nil {
		log.Printf("otel: failed to create flow.entry_to_terminal.duration histogram: %v", err)
	}
}

// RecordValidationStep records a single validation step's pass/fail outcome as a nested span
// (under the caller's ctx, which carries the flow's root span) and increments the
// flow.validation.outcomes counter. It never alters control flow or the passed-in error.
// The span's own context (spanCtx) - not the original ctx - is propagated to the calls that
// record against this span, so they nest under it correctly.
func RecordValidationStep(ctx context.Context, flow, step string, stepErr error) {
	spanCtx, span := Tracer.Start(ctx, "validate."+step)
	defer span.End()

	outcome := "pass"
	if stepErr != nil {
		outcome = "fail"
	}

	span.SetAttributes(
		attribute.String("flow", flow),
		attribute.String("step", step),
		attribute.String("validation.outcome", outcome),
	)

	FlowValidationOutcomes.Add(spanCtx, 1, metric.WithAttributes(
		attribute.String("flow", flow),
		attribute.String("step", step),
		attribute.String("outcome", outcome),
	))
}
