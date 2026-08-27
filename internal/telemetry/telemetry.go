package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Tracer is the single tracer used for business flow spans across the service.
// It must be created from the global TracerProvider so that it starts
// emitting real spans once that provider is registered at startup (see
// config.InitOTel, invoked from cmd/web/main.go).
var Tracer = otel.Tracer("golang-clean-architecture")

// meter is the single package-level meter for this service. Every instrument
// below MUST be created from this same meter so that any future observable
// callbacks registered against it fire correctly and metric names are never
// double-defined from different meters.
var meter = otel.Meter("golang-clean-architecture")

var (
	// FlowEntries counts every invocation of a business flow's entry point,
	// independent of its eventual outcome (throughput SLI).
	FlowEntries metric.Int64Counter

	// FlowOutcomes counts the terminal outcome of a business flow, tagged by
	// the "outcome" attribute (success|failure) (success-rate SLI).
	FlowOutcomes metric.Int64Counter

	// FlowDuration records the end-to-end duration, in seconds, of a business
	// flow from entry to terminal outcome (latency/freshness SLIs).
	FlowDuration metric.Float64Histogram

	// ValidationOutcomes counts the pass/fail outcome of each validation step
	// within a business flow (validation-failure-rate SLI).
	ValidationOutcomes metric.Int64Counter
)

// InitInstruments creates every metric instrument from the single package
// meter above. It must be called once during application startup, after the
// global MeterProvider has been registered (see config.InitOTel).
func InitInstruments() error {
	var err error

	FlowEntries, err = meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Number of times a business flow entry point was invoked"),
	)
	if err != nil {
		return err
	}

	FlowOutcomes, err = meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Terminal outcomes of business flows, tagged by outcome"),
	)
	if err != nil {
		return err
	}

	FlowDuration, err = meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of business flows"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	ValidationOutcomes, err = meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Per-step validation outcomes within a business flow"),
	)
	if err != nil {
		return err
	}

	return nil
}
