package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Meter and Tracer are the single package-level meter/tracer used by every
// instrument and span in this service (one meter/tracer per service).
var (
	Meter  = otel.Meter("golang-clean-architecture")
	Tracer = otel.Tracer("golang-clean-architecture")
)

var (
	// FlowEntries counts every invocation of the async Kafka worker flow's
	// entry point, independent of eventual outcome (throughput SLI).
	FlowEntries metric.Int64Counter
	// FlowOutcomes is the terminal-outcome counter for the whole flow
	// (success rate / abandonment SLIs). Use the "outcome" attribute.
	FlowOutcomes metric.Int64Counter
	// FlowDuration records end-to-end flow latency in seconds (latency SLI).
	FlowDuration metric.Float64Histogram
	// FlowValidationOutcomes records the pass/fail outcome of each
	// validation step in the flow (validation failure rate SLI).
	FlowValidationOutcomes metric.Int64Counter
	// FlowEntryToTerminalDuration records wall-clock time between flow
	// entry and its terminal state, including async hops (freshness SLI).
	FlowEntryToTerminalDuration metric.Float64Histogram
	// FlowTerminalOutcomes counts terminal outcomes reached by the Kafka
	// worker (the actual async terminal point of the flow), distinct from
	// the synchronous HTTP-boundary FlowOutcomes. Entries without a
	// matching worker terminal represent abandonment.
	FlowTerminalOutcomes metric.Int64Counter
)

func init() {
	var err error

	FlowEntries, err = Meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Count of invocations of the async processing business flow entry point"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.entries counter: %w", err))
	}

	FlowOutcomes, err = Meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Terminal outcomes of the async processing business flow"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.outcomes counter: %w", err))
	}

	FlowDuration, err = Meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of the async processing business flow"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.duration histogram: %w", err))
	}

	FlowValidationOutcomes, err = Meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Outcome of each validation step within the async processing business flow"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.validation.outcomes counter: %w", err))
	}

	FlowEntryToTerminalDuration, err = Meter.Float64Histogram(
		"flow.entry_to_terminal.duration",
		metric.WithDescription("Wall-clock time between flow entry and terminal state, including async hops"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.entry_to_terminal.duration histogram: %w", err))
	}

	FlowTerminalOutcomes, err = Meter.Int64Counter(
		"flow.terminal.outcomes",
		metric.WithDescription("Terminal outcomes of the async processing business flow as observed by the Kafka worker"),
	)
	if err != nil {
		panic(fmt.Errorf("telemetry: failed to create flow.terminal.outcomes counter: %w", err))
	}
}

// Setup builds and registers the global OpenTelemetry TracerProvider and
// MeterProvider for this application. Go has no bytecode agent, so the
// application always owns and registers the SDK; call this exactly once
// from main() and defer the returned shutdown func so buffered telemetry
// flushes on exit. The OTLP endpoint is env-driven via the standard
// OTEL_EXPORTER_OTLP_ENDPOINT variable, consumed by the otlptracegrpc and
// otlpmetricgrpc clients by default.
func Setup(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to build resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create otlp trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create otlp metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if shutdownErr := tracerProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			errs = append(errs, shutdownErr)
		}
		if shutdownErr := meterProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			errs = append(errs, shutdownErr)
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry: shutdown errors: %v", errs)
		}
		return nil
	}

	return shutdown, nil
}
