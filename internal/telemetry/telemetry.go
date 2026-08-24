// Package telemetry owns the single OpenTelemetry meter/tracer for this
// service and every business-flow instrument recorded against them. Do not
// call otel.Meter/otel.Tracer anywhere else in this repo — instruments and
// callbacks must all come from these package-level handles so they observe
// the same registered providers.
package telemetry

import (
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const instrumentationScope = "golang-clean-architecture"

// EntryTimestampHeaderKey is the Kafka message header used to carry the
// wall-clock time the flow entered the system (message produced), so a
// consumer anywhere downstream can compute entry-to-terminal duration even
// though it never shared a live span with the producer.
const EntryTimestampHeaderKey = "flow-entry-ts"

// AbandonmentThreshold is the maximum time a flow instance may spend between
// its entry event and reaching a terminal state before that terminal state
// is counted as "abandoned" instead of "success".
const AbandonmentThreshold = 30 * time.Minute

var (
	// Tracer is the single tracer used for every business-flow span.
	Tracer = otel.Tracer(instrumentationScope)
	meter  = otel.Meter(instrumentationScope)

	// FlowEntries counts every invocation of a flow's entry point
	// (a Kafka message produced), independent of its eventual outcome.
	FlowEntries metric.Int64Counter

	// FlowOutcomes counts terminal flow outcomes, labelled by
	// outcome=success|failed|abandoned and flow=<topic>.
	FlowOutcomes metric.Int64Counter

	// FlowValidationOutcomes counts per-step validation outcomes within a
	// flow, labelled by outcome=passed|failed and flow=<topic>.
	FlowValidationOutcomes metric.Int64Counter

	// FlowDuration records the end-to-end flow latency (entry to terminal
	// state), in seconds.
	FlowDuration metric.Float64Histogram

	// FlowEntryToTerminalDuration records the wall-clock time between a
	// flow's entry event and its terminal state transition, in seconds
	// (freshness SLI).
	FlowEntryToTerminalDuration metric.Float64Histogram
)

func init() {
	var err error

	FlowEntries, err = meter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Count of business flow entry-point invocations (e.g. a Kafka message produced)"),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		panic(err)
	}

	FlowOutcomes, err = meter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Count of terminal business flow outcomes"),
		metric.WithUnit("{outcome}"),
	)
	if err != nil {
		panic(err)
	}

	FlowValidationOutcomes, err = meter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Count of per-step validation outcomes within a business flow"),
		metric.WithUnit("{outcome}"),
	)
	if err != nil {
		panic(err)
	}

	FlowDuration, err = meter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of the business flow, from entry to terminal state"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(err)
	}

	FlowEntryToTerminalDuration, err = meter.Float64Histogram(
		"flow.entry_to_terminal.duration",
		metric.WithDescription("Wall-clock time between the flow's entry event and its terminal state transition"),
		metric.WithUnit("s"),
	)
	if err != nil {
		panic(err)
	}
}

// StampEntryTimestamp records the flow's entry time on an outbound Kafka
// message so any downstream consumer can compute entry-to-terminal duration.
func StampEntryTimestamp(msg *sarama.ProducerMessage, t time.Time) {
	msg.Headers = append(msg.Headers, sarama.RecordHeader{
		Key:   []byte(EntryTimestampHeaderKey),
		Value: []byte(strconv.FormatInt(t.UnixNano(), 10)),
	})
}

// ExtractEntryTimestamp reads the flow's entry time from a consumed Kafka
// message. It returns the zero Time if the header is absent or malformed.
func ExtractEntryTimestamp(msg *sarama.ConsumerMessage) time.Time {
	for _, h := range msg.Headers {
		if h != nil && string(h.Key) == EntryTimestampHeaderKey {
			nanos, err := strconv.ParseInt(string(h.Value), 10, 64)
			if err != nil {
				return time.Time{}
			}
			return time.Unix(0, nanos)
		}
	}
	return time.Time{}
}

// ProducerHeaderCarrier adapts a sarama.ProducerMessage's headers to
// propagation.TextMapCarrier so trace context can be injected before send.
type ProducerHeaderCarrier struct {
	Msg *sarama.ProducerMessage
}

func (c ProducerHeaderCarrier) Get(key string) string {
	for _, h := range c.Msg.Headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c ProducerHeaderCarrier) Set(key, value string) {
	c.Msg.Headers = append(c.Msg.Headers, sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c ProducerHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Msg.Headers))
	for _, h := range c.Msg.Headers {
		keys = append(keys, string(h.Key))
	}
	return keys
}

// ConsumerHeaderCarrier adapts a sarama.ConsumerMessage's headers to
// propagation.TextMapCarrier so trace context can be extracted on receipt.
type ConsumerHeaderCarrier struct {
	Msg *sarama.ConsumerMessage
}

func (c ConsumerHeaderCarrier) Get(key string) string {
	for _, h := range c.Msg.Headers {
		if h != nil && string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c ConsumerHeaderCarrier) Set(key, value string) {
	c.Msg.Headers = append(c.Msg.Headers, &sarama.RecordHeader{
		Key:   []byte(key),
		Value: []byte(value),
	})
}

func (c ConsumerHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Msg.Headers))
	for _, h := range c.Msg.Headers {
		if h != nil {
			keys = append(keys, string(h.Key))
		}
	}
	return keys
}
