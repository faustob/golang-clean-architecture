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

type AddressConsumer struct {
	Log *logrus.Logger
}

func NewAddressConsumer(log *logrus.Logger) *AddressConsumer {
	return &AddressConsumer{
		Log: log,
	}
}

func (c AddressConsumer) Consume(ctx context.Context, message *sarama.ConsumerMessage) error {
	validateCtx, validateSpan := telemetry.Tracer.Start(ctx, "flow.validate addresses")
	defer validateSpan.End()

	addressEvent := new(model.AddressEvent)
	if err := json.Unmarshal(message.Value, addressEvent); err != nil {
		c.Log.WithError(err).Error("error unmarshalling address event")
		validateSpan.SetAttributes(attribute.Bool("validation.passed", false))
		validateSpan.RecordError(err)
		telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
			attribute.String("flow", "addresses"),
			attribute.String("outcome", "failed"),
		))
		return err
	}
	validateSpan.SetAttributes(attribute.Bool("validation.passed", true))
	telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
		attribute.String("flow", "addresses"),
		attribute.String("outcome", "passed"),
	))

	// TODO process event
	c.Log.Infof("Received topic addresses with event: %v from partition %d", addressEvent, message.Partition)
	return nil
}
