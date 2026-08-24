package messaging

import (
	"context"
	"encoding/json"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/telemetry"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ContactConsumer struct {
	Log *logrus.Logger
}

func NewContactConsumer(log *logrus.Logger) *ContactConsumer {
	return &ContactConsumer{
		Log: log,
	}
}

func (c ContactConsumer) Consume(ctx context.Context, message *sarama.ConsumerMessage) error {
	validateCtx, validateSpan := telemetry.Tracer.Start(ctx, "flow.validate contacts")
	defer validateSpan.End()

	ContactEvent := new(model.ContactEvent)
	if err := json.Unmarshal(message.Value, ContactEvent); err != nil {
		c.Log.WithError(err).Error("error unmarshalling Contact event")
		validateSpan.SetAttributes(attribute.Bool("validation.passed", false))
		validateSpan.RecordError(err)
		telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
			attribute.String("flow", "contacts"),
			attribute.String("outcome", "failed"),
		))
		return err
	}
	validateSpan.SetAttributes(attribute.Bool("validation.passed", true))
	telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
		attribute.String("flow", "contacts"),
		attribute.String("outcome", "passed"),
	))

	// TODO process event
	c.Log.Infof("Received topic contacts with event: %v from partition %d", ContactEvent, message.Partition)
	return nil
}
