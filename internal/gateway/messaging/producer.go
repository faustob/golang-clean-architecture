package messaging

import (
	"context"
	"encoding/json"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/telemetry"
	"time"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Producer[T model.Event] struct {
	Producer sarama.SyncProducer
	Topic    string
	Log      *logrus.Logger
}

func (p *Producer[T]) GetTopic() *string {
	return &p.Topic
}

func (p *Producer[T]) Send(event T) error {
	ctx := context.Background()

	// Flow-entry counter: fired independent of the eventual outcome (throughput SLI).
	telemetry.FlowEntries.Add(ctx, 1, metric.WithAttributes(attribute.String("flow", p.Topic)))

	// Root span for the whole business flow; its trace id is stamped into the
	// Kafka headers below so the consumer's span rolls back into this one.
	ctx, span := telemetry.Tracer.Start(ctx, "flow.produce "+p.Topic, trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", p.Topic),
		attribute.String("flow", p.Topic),
	))
	defer span.End()

	value, err := json.Marshal(event)
	if err != nil {
		p.Log.WithError(err).Error("failed to marshal event")
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal_error")
		return err
	}

	message := &sarama.ProducerMessage{
		Topic: p.Topic,
		Key:   sarama.StringEncoder(event.GetId()),
		Value: sarama.ByteEncoder(value),
	}

	// End-to-end correlation key: propagate trace context and the flow's entry
	// timestamp via message headers so downstream consumers roll back into
	// this span and can compute entry-to-terminal duration.
	otel.GetTextMapPropagator().Inject(ctx, telemetry.ProducerHeaderCarrier{Msg: message})
	telemetry.StampEntryTimestamp(message, time.Now())

	partition, offset, err := p.Producer.SendMessage(message)
	if err != nil {
		p.Log.WithError(err).Error("failed to produce message")
		span.RecordError(err)
		span.SetStatus(codes.Error, "send_error")
		return err
	}

	p.Log.Debugf("Message sent to topic %s, partition %d, offset %d", p.Topic, partition, offset)
	return nil
}
