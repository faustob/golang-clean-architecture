package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// scopeName/flowName identify the instrumentation scope and the business flow (user CRUD via
// the HTTP API: register/login/current/logout/update) that these instruments measure.
const (
	scopeName = "golang-clean-architecture/user-flow"
	flowName  = "user-crud"
)

// One meter/tracer for this package -- every instrument below is created from it.
var (
	tracer = otel.Tracer(scopeName)
	meter  = otel.Meter(scopeName)

	// FlowEntries counts every invocation of a user-crud flow entry point, independent of outcome.
	FlowEntries metric.Int64Counter
	// FlowOutcomes counts the terminal outcome (success/error) of each user-crud flow instance.
	FlowOutcomes metric.Int64Counter
	// FlowDuration records the end-to-end duration of a user-crud flow instance, in seconds.
	FlowDuration metric.Float64Histogram
	// FlowEntryToTerminalDuration records the wall-clock time between the flow's entry event and
	// its terminal state transition (freshness); for this synchronous request/response flow that
	// span coincides with FlowDuration, but it is recorded as its own named instrument because it
	// backs a distinct SLI (Flow Completion Freshness) and downstream queries key off its name.
	FlowEntryToTerminalDuration metric.Float64Histogram
	// FlowValidationOutcomes counts the pass/fail outcome of a validation step within a flow instance.
	FlowValidationOutcomes metric.Int64Counter
)

func init() {
	var err error

	if FlowEntries, err = meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Number of times the user CRUD flow entry point was invoked"),
	); err != nil {
		otel.Handle(err)
	}

	if FlowOutcomes, err = meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Terminal outcome of the user CRUD flow"),
	); err != nil {
		otel.Handle(err)
	}

	if FlowDuration, err = meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of the user CRUD flow"),
		metric.WithUnit("s"),
	); err != nil {
		otel.Handle(err)
	}

	if FlowEntryToTerminalDuration, err = meter.Float64Histogram(
		"flow.entry_to_terminal.duration",
		metric.WithDescription("Wall-clock time between the user CRUD flow's entry event and its terminal state transition"),
		metric.WithUnit("s"),
	); err != nil {
		otel.Handle(err)
	}

	if FlowValidationOutcomes, err = meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Outcome of a validation step within the user CRUD flow"),
	); err != nil {
		otel.Handle(err)
	}
}

// StartFlow begins the root span for one instance of the user-crud business flow (one HTTP
// request handled by the user controller) and increments the flow-entry counter. step
// identifies the entry point, e.g. "register", "login", "current", "logout", "update".
// The caller must `defer span.End()` and later call EndFlow with the same span/start/step.
func StartFlow(ctx context.Context, step string) (context.Context, trace.Span, time.Time) {
	flowCtx, span := tracer.Start(ctx, "user_flow."+step, trace.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
	))
	FlowEntries.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
	))
	return flowCtx, span, time.Now()
}

// EndFlow records the terminal outcome, the end-to-end duration, and the entry-to-terminal
// (freshness) duration of the flow instance started by StartFlow. It does NOT end the span --
// the caller's `defer span.End()` does that -- it only annotates it with error status/type when
// err is non-nil.
func EndFlow(ctx context.Context, span trace.Span, start time.Time, step string, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
	}
	FlowOutcomes.Add(ctx, 1, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
		attribute.String("outcome", outcome),
	))

	elapsed := time.Since(start).Seconds()
	FlowDuration.Record(ctx, elapsed, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
	))

	// This flow is synchronous end-to-end (a single HTTP request/response terminates it), so the
	// entry-to-terminal (freshness) duration is recorded here, at the terminal state transition,
	// using the same start timestamp captured at flow entry.
	FlowEntryToTerminalDuration.Record(ctx, elapsed, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
		attribute.String("outcome", outcome),
	))
}

// RecordValidation emits a nested span plus a pass/fail counter for a single validation step
// (identified by step, e.g. "register"/"login"/...) within the current user-crud flow instance;
// ctx should be the flow context so the span nests under the flow's root span.
func RecordValidation(ctx context.Context, step string, err error) {
	ctx, span := tracer.Start(ctx, "validate."+step)
	defer span.End()

	outcome := "passed"
	if err != nil {
		outcome = "failed"
		span.SetAttributes(attribute.Bool("validation.passed", false))
	} else {
		span.SetAttributes(attribute.Bool("validation.passed", true))
	}

	FlowValidationOutcomes.Add(ctx, 1, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("flow.step", step),
		attribute.String("outcome", outcome),
	))
}
