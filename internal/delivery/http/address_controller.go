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
	auth := middleware.GetUser(ctx)

	telemetry.FlowEntries.Add(ctx.UserContext(), 1, metric.WithAttributes(attribute.String("flow", "address_management")))
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.create")
	defer span.End()
	start := time.Now()

	request := new(model.CreateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		telemetry.ValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("step", "body_parse"), attribute.String("outcome", "failed")))
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "invalid request body")
		return fiber.ErrBadRequest
	}
	telemetry.ValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("step", "body_parse"), attribute.String("outcome", "passed")))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")

	response, err := c.UseCase.Create(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to create address")
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "use case failed")
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) List(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")

	telemetry.FlowEntries.Add(ctx.UserContext(), 1, metric.WithAttributes(attribute.String("flow", "address_management")))
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.list")
	defer span.End()
	start := time.Now()

	request := &model.ListAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
	}

	responses, err := c.UseCase.List(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to list addresses")
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "use case failed")
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))

	return ctx.JSON(model.WebResponse[[]model.AddressResponse]{Data: responses})
}

func (c *AddressController) Get(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	telemetry.FlowEntries.Add(ctx.UserContext(), 1, metric.WithAttributes(attribute.String("flow", "address_management")))
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.get")
	defer span.End()
	start := time.Now()

	request := &model.GetAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	response, err := c.UseCase.Get(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to get address")
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "use case failed")
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Update(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)

	telemetry.FlowEntries.Add(ctx.UserContext(), 1, metric.WithAttributes(attribute.String("flow", "address_management")))
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.update")
	defer span.End()
	start := time.Now()

	request := new(model.UpdateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		telemetry.ValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("step", "body_parse"), attribute.String("outcome", "failed")))
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "invalid request body")
		return fiber.ErrBadRequest
	}
	telemetry.ValidationOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("step", "body_parse"), attribute.String("outcome", "passed")))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")
	request.ID = ctx.Params("addressId")

	response, err := c.UseCase.Update(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to update address")
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "use case failed")
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Delete(ctx *fiber.Ctx) error {
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	telemetry.FlowEntries.Add(ctx.UserContext(), 1, metric.WithAttributes(attribute.String("flow", "address_management")))
	flowCtx, span := telemetry.Tracer.Start(ctx.UserContext(), "flow.address.delete")
	defer span.End()
	start := time.Now()

	request := &model.DeleteAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	if err := c.UseCase.Delete(flowCtx, request); err != nil {
		c.Log.WithError(err).Error("failed to delete address")
		telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "failure")))
		telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))
		span.SetStatus(codes.Error, "use case failed")
		return err
	}

	telemetry.FlowOutcomes.Add(flowCtx, 1, metric.WithAttributes(attribute.String("flow", "address_management"), attribute.String("outcome", "success")))
	telemetry.FlowDuration.Record(flowCtx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("flow", "address_management")))

	return ctx.JSON(model.WebResponse[bool]{Data: true})
}
