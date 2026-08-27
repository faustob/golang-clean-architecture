package http

import (
	"time"

	"golang-clean-architecture/internal/delivery/http/middleware"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/telemetry"
	"golang-clean-architecture/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

type AddressController struct {
	UseCase *usecase.AddressUseCase
	Log     *logrus.Logger
}

func NewAddressController(useCase *usecase.AddressUseCase, log *logrus.Logger) *AddressController {
	return &AddressController{
		Log:     log,
		UseCase: useCase,
	}
}

func (c *AddressController) Create(ctx *fiber.Ctx) error {
	// Root span for the whole async Kafka worker flow; propagated via the
	// context passed to the use case so downstream publishing can stamp the
	// trace id for the worker to roll back into. flowStart is also stamped
	// onto the outgoing Kafka message (via the use case/producer) so the
	// worker can compute entry-to-terminal freshness across the async hop.
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.create")
	defer span.End()

	flowAttr := attribute.String("flow", "address.create")
	flowStart := time.Now()
	telemetry.FlowEntries.Add(flowCtx, 1, metric.WithAttributes(flowAttr))

	auth := middleware.GetUser(ctx)

	request := new(model.CreateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		telemetry.FlowValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(flowAttr, attribute.String("outcome", "failed")))
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(flowAttr, attribute.String("outcome", "validation_failed")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))
		telemetry.FlowEntryToTerminalDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))
		span.SetAttributes(attribute.String("validation.outcome", "failed"))
		span.SetStatus(codes.Error, "validation failed")
		return fiber.ErrBadRequest
	}
	telemetry.FlowValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(flowAttr, attribute.String("outcome", "passed")))
	span.SetAttributes(attribute.String("validation.outcome", "passed"))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")

	response, err := c.UseCase.Create(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to create address")
		span.SetStatus(codes.Error, err.Error())
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(flowAttr, attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))
		telemetry.FlowEntryToTerminalDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(flowAttr, attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))
	telemetry.FlowEntryToTerminalDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(flowAttr))

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) List(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")

	request := &model.ListAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
	}

	responses, err := c.UseCase.List(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to list addresses")
		return err
	}

	return ctx.JSON(model.WebResponse[[]model.AddressResponse]{Data: responses})
}

func (c *AddressController) Get(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	request := &model.GetAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	response, err := c.UseCase.Get(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to get address")
		return err
	}

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Update(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	request := new(model.UpdateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		return fiber.ErrBadRequest
	}

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")
	request.ID = ctx.Params("addressId")

	response, err := c.UseCase.Update(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to update address")
		return err
	}

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Delete(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	request := &model.DeleteAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	if err := c.UseCase.Delete(ctx.UserContext(), request); err != nil {
		c.Log.WithError(err).Error("failed to delete address")
		return err
	}

	return ctx.JSON(model.WebResponse[bool]{Data: true})
}
