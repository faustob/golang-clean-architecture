package telemetry

import (
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const scopeName = "golang-clean-architecture/flow"

// Tracer is the single package-level tracer used for business-flow spans
// across the service.
var Tracer trace.Tracer = otel.Tracer(scopeName)

// meter is the single package-level meter; every flow instrument MUST be
// created from this meter.
var meter = otel.Meter(scopeName)

var (
	// FlowEntries counts every invocation of a business flow's entry point,
	// independent of eventual outcome (used for throughput SLIs).
	FlowEntries metric.Int64Counter

	// FlowOutcomes counts terminal outcomes ("success", "failed", "abandoned")
	// of a business flow (used for success-rate / abandonment SLIs).
	FlowOutcomes metric.Int64Counter

	// FlowDuration records, in seconds, the duration from flow entry to the
	// terminal outcome observed by this service (used for latency/freshness
	// SLIs).
	FlowDuration metric.Float64Histogram

	// FlowValidationOutcomes counts per-step validation outcomes ("passed",
	// "failed") within a business flow (used for validation-failure-rate
	// SLIs).
	FlowValidationOutcomes metric.Int64Counter
)

func init() {
	var err error

	FlowEntries, err = meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Number of times an async business flow entry point was invoked"),
	)
	if err != nil {
		log.Printf("telemetry: failed to create flow.entries counter: %v", err)
	}

	FlowOutcomes, err = meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Terminal outcomes of an async business flow"),
	)
	if err != nil {
		log.Printf("telemetry: failed to create flow.outcomes counter: %v", err)
	}

	FlowDuration, err = meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of an async business flow, observed by this service"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Printf("telemetry: failed to create flow.duration histogram: %v", err)
	}

	FlowValidationOutcomes, err = meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Per-step validation outcomes within an async business flow"),
	)
	if err != nil {
		log.Printf("telemetry: failed to create flow.validation.outcomes counter: %v", err)
	}
}
