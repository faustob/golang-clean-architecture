package messaging

import (
	"context"
	"time"

	"github.com/IBM/sarama"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"golang-clean-architecture/internal/telemetry"
)

type ConsumerHandler func(ctx context.Context, message *sarama.ConsumerMessage) error

type ConsumerGroupHandler struct {
	Handler ConsumerHandler
	Log     *logrus.Logger
}

func (h *ConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			flow := claim.Topic()
			entryTs := telemetry.ExtractEntryTimestamp(message)
			msgCtx := otel.GetTextMapPropagator().Extract(context.Background(), telemetry.ConsumerHeaderCarrier{Msg: message})
			msgCtx, span := telemetry.Tracer.Start(msgCtx, "flow.consume "+flow, trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination.name", flow),
				attribute.String("flow", flow),
			))

			err := h.Handler(msgCtx, message)

			// Terminal outcome for the E2E flow success-rate / abandonment SLIs.
			outcome := "success"
			if err != nil {
				outcome = "failed"
				span.RecordError(err)
				span.SetStatus(codes.Error, "handler_error")
			} else if !entryTs.IsZero() {
				elapsed := time.Since(entryTs)
				if elapsed > telemetry.AbandonmentThreshold {
					// Reached a terminal state, but only after the flow's SLA window
					// elapsed since entry: count it as abandoned, not successful.
					outcome = "abandoned"
				}
				telemetry.FlowDuration.Record(msgCtx, elapsed.Seconds(), metric.WithAttributes(attribute.String("flow", flow)))
				telemetry.FlowEntryToTerminalDuration.Record(msgCtx, elapsed.Seconds(), metric.WithAttributes(attribute.String("flow", flow)))
			}
			telemetry.FlowOutcomes.Add(msgCtx, 1, metric.WithAttributes(
				attribute.String("flow", flow),
				attribute.String("outcome", outcome),
			))
			span.SetAttributes(attribute.String("flow.outcome", outcome))
			span.End()

			if err != nil {
				h.Log.WithError(err).Error("Failed to process message")
			} else {
				session.MarkMessage(message, "")
			}

		case <-session.Context().Done():
			return nil
		}
	}
}

func ConsumeTopic(ctx context.Context, consumerGroup sarama.ConsumerGroup, topic string, log *logrus.Logger, handler ConsumerHandler) {
	consumerHandler := &ConsumerGroupHandler{
		Handler: handler,
		Log:     log,
	}

	go func() {
		for {
			if err := consumerGroup.Consume(ctx, []string{topic}, consumerHandler); err != nil {
				log.WithError(err).Error("Error from consumer")
			}

			if ctx.Err() != nil {
				log.Info("Context cancelled, stopping consumer")
				return
			}
		}
	}()

	go func() {
		for err := range consumerGroup.Errors() {
			log.WithError(err).Error("Consumer group error")
		}
	}()

	<-ctx.Done()
	log.Infof("Closing consumer group for topic: %s", topic)
	if err := consumerGroup.Close(); err != nil {
		log.WithError(err).Error("Error closing consumer group")
	}
}
