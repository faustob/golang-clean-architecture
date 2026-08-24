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

type UserConsumer struct {
	Log *logrus.Logger
}

func NewUserConsumer(log *logrus.Logger) *UserConsumer {
	return &UserConsumer{
		Log: log,
	}
}

func (c UserConsumer) Consume(ctx context.Context, message *sarama.ConsumerMessage) error {
	validateCtx, validateSpan := telemetry.Tracer.Start(ctx, "flow.validate users")
	defer validateSpan.End()

	UserEvent := new(model.UserEvent)
	if err := json.Unmarshal(message.Value, UserEvent); err != nil {
		c.Log.WithError(err).Error("error unmarshalling User event")
		validateSpan.SetAttributes(attribute.Bool("validation.passed", false))
		validateSpan.RecordError(err)
		telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
			attribute.String("flow", "users"),
			attribute.String("outcome", "failed"),
		))
		return err
	}
	validateSpan.SetAttributes(attribute.Bool("validation.passed", true))
	telemetry.FlowValidationOutcomes.Add(validateCtx, 1, metric.WithAttributes(
		attribute.String("flow", "users"),
		attribute.String("outcome", "passed"),
	))

	// TODO process event
	c.Log.Infof("Received topic users with event: %v from partition %d", UserEvent, message.Partition)
	return nil
}
