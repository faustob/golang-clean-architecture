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
	"go.opentelemetry.io/otel/trace"
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
	auth := middleware.GetUser(ctx)

	const flowName = "address.create"
	flowCtx, flowSpan := telemetry.Tracer.Start(ctx.UserContext(), "flow."+flowName,
		trace.WithAttributes(attribute.String("flow.name", flowName)))
	defer flowSpan.End()

	flowStart := time.Now()
	telemetry.FlowEntries.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", flowName)))

	recordOutcome := func(outcome string) {
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", flowName),
			attribute.String("outcome", outcome),
		))
		telemetry.FlowDuration.Record(flowCtx, time.Since(flowStart).Seconds(), metric.WithAttributes(
			attribute.String("flow", flowName),
			attribute.String("outcome", outcome),
		))
	}

	request := new(model.CreateAddressRequest)
	_, validationSpan := telemetry.Tracer.Start(flowCtx, "flow."+flowName+".validate")
	if err := ctx.BodyParser(request); err != nil {
		validationSpan.SetAttributes(attribute.Bool("validation.passed", false))
		validationSpan.RecordError(err)
		validationSpan.End()
		telemetry.FlowValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", flowName),
			attribute.String("outcome", "failed"),
		))
		flowSpan.SetStatus(codes.Error, "validation failed")
		recordOutcome("failed")
		c.Log.WithError(err).Error("failed to parse request body")
		return fiber.ErrBadRequest
	}
	validationSpan.SetAttributes(attribute.Bool("validation.passed", true))
	validationSpan.End()
	telemetry.FlowValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", flowName),
		attribute.String("outcome", "passed"),
	))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")

	response, err := c.UseCase.Create(flowCtx, request)
	if err != nil {
		flowSpan.RecordError(err)
		flowSpan.SetStatus(codes.Error, "flow failed")
		recordOutcome("failed")
		c.Log.WithError(err).Error("failed to create address")
		return err
	}

	recordOutcome("success")
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
