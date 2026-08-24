package http

import (
	"golang-clean-architecture/internal/delivery/http/middleware"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/usecase"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

var (
	addressFlowTracer = otel.Tracer("golang-clean-architecture/address-flow")
	addressFlowMeter  = otel.Meter("golang-clean-architecture/address-flow")

	flowEntryCounter       metric.Int64Counter
	flowOutcomeCounter     metric.Int64Counter
	flowValidationCounter  metric.Int64Counter
	flowDurationHistogram  metric.Float64Histogram
	flowFreshnessHistogram metric.Float64Histogram
)

func init() {
	var instErr error

	flowEntryCounter, instErr = addressFlowMeter.Int64Counter(
		"flow.entries",
		metric.WithDescription("Count of address management flow entries"),
	)
	if instErr != nil {
		logrus.WithError(instErr).Error("failed to create flow.entries counter")
	}

	flowOutcomeCounter, instErr = addressFlowMeter.Int64Counter(
		"flow.outcomes",
		metric.WithDescription("Count of address management flow terminal outcomes"),
	)
	if instErr != nil {
		logrus.WithError(instErr).Error("failed to create flow.outcomes counter")
	}

	flowValidationCounter, instErr = addressFlowMeter.Int64Counter(
		"flow.validation.outcomes",
		metric.WithDescription("Count of address management flow per-step validation outcomes"),
	)
	if instErr != nil {
		logrus.WithError(instErr).Error("failed to create flow.validation.outcomes counter")
	}

	flowDurationHistogram, instErr = addressFlowMeter.Float64Histogram(
		"flow.duration",
		metric.WithDescription("End-to-end duration of the address management flow"),
		metric.WithUnit("s"),
	)
	if instErr != nil {
		logrus.WithError(instErr).Error("failed to create flow.duration histogram")
	}

	flowFreshnessHistogram, instErr = addressFlowMeter.Float64Histogram(
		"flow.entry_to_terminal.duration",
		metric.WithDescription("Wall-clock time between address management flow entry and terminal state"),
		metric.WithUnit("s"),
	)
	if instErr != nil {
		logrus.WithError(instErr).Error("failed to create flow.entry_to_terminal.duration histogram")
	}
}

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
	start := time.Now()
	flowCtx, span := addressFlowTracer.Start(ctx.UserContext(), "address.flow.create")
	defer span.End()

	flowEntryCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_create"),
	))

	var flowErr error
	outcome := "success"
	defer func() {
		if flowErr != nil {
			outcome = "failure"
			span.RecordError(flowErr)
			span.SetStatus(codes.Error, flowErr.Error())
		}
		flowOutcomeCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_create"),
			attribute.String("outcome", outcome),
		))
		elapsed := time.Since(start).Seconds()
		flowDurationHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_create"),
		))
		flowFreshnessHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_create"),
		))
	}()

	auth := middleware.GetUser(ctx)

	request := new(model.CreateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		flowValidationCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_create"),
			attribute.String("step", "body_parse"),
			attribute.String("outcome", "failed"),
		))
		flowErr = fiber.ErrBadRequest
		return flowErr
	}
	flowValidationCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_create"),
		attribute.String("step", "body_parse"),
		attribute.String("outcome", "passed"),
	))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")

	response, err := c.UseCase.Create(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to create address")
		flowErr = err
		return flowErr
	}

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) List(ctx *fiber.Ctx) error {
	start := time.Now()
	flowCtx, span := addressFlowTracer.Start(ctx.UserContext(), "address.flow.list")
	defer span.End()

	flowEntryCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_list"),
	))

	var flowErr error
	outcome := "success"
	defer func() {
		if flowErr != nil {
			outcome = "failure"
			span.RecordError(flowErr)
			span.SetStatus(codes.Error, flowErr.Error())
		}
		flowOutcomeCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_list"),
			attribute.String("outcome", outcome),
		))
		elapsed := time.Since(start).Seconds()
		flowDurationHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_list"),
		))
		flowFreshnessHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_list"),
		))
	}()

	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")

	request := &model.ListAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
	}

	responses, err := c.UseCase.List(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to list addresses")
		flowErr = err
		return flowErr
	}

	return ctx.JSON(model.WebResponse[[]model.AddressResponse]{Data: responses})
}

func (c *AddressController) Get(ctx *fiber.Ctx) error {
	start := time.Now()
	flowCtx, span := addressFlowTracer.Start(ctx.UserContext(), "address.flow.get")
	defer span.End()

	flowEntryCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_get"),
	))

	var flowErr error
	outcome := "success"
	defer func() {
		if flowErr != nil {
			outcome = "failure"
			span.RecordError(flowErr)
			span.SetStatus(codes.Error, flowErr.Error())
		}
		flowOutcomeCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_get"),
			attribute.String("outcome", outcome),
		))
		elapsed := time.Since(start).Seconds()
		flowDurationHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_get"),
		))
		flowFreshnessHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_get"),
		))
	}()

	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	request := &model.GetAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	response, err := c.UseCase.Get(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to get address")
		flowErr = err
		return flowErr
	}

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Update(ctx *fiber.Ctx) error {
	start := time.Now()
	flowCtx, span := addressFlowTracer.Start(ctx.UserContext(), "address.flow.update")
	defer span.End()

	flowEntryCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_update"),
	))

	var flowErr error
	outcome := "success"
	defer func() {
		if flowErr != nil {
			outcome = "failure"
			span.RecordError(flowErr)
			span.SetStatus(codes.Error, flowErr.Error())
		}
		flowOutcomeCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_update"),
			attribute.String("outcome", outcome),
		))
		elapsed := time.Since(start).Seconds()
		flowDurationHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_update"),
		))
		flowFreshnessHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_update"),
		))
	}()

	auth := middleware.GetUser(ctx)

	request := new(model.UpdateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		flowValidationCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_update"),
			attribute.String("step", "body_parse"),
			attribute.String("outcome", "failed"),
		))
		flowErr = fiber.ErrBadRequest
		return flowErr
	}
	flowValidationCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_update"),
		attribute.String("step", "body_parse"),
		attribute.String("outcome", "passed"),
	))

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")
	request.ID = ctx.Params("addressId")

	response, err := c.UseCase.Update(flowCtx, request)
	if err != nil {
		c.Log.WithError(err).Error("failed to update address")
		flowErr = err
		return flowErr
	}

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Delete(ctx *fiber.Ctx) error {
	start := time.Now()
	flowCtx, span := addressFlowTracer.Start(ctx.UserContext(), "address.flow.delete")
	defer span.End()

	flowEntryCounter.Add(flowCtx, 1, metric.WithAttributes(
		attribute.String("flow", "address_delete"),
	))

	var flowErr error
	outcome := "success"
	defer func() {
		if flowErr != nil {
			outcome = "failure"
			span.RecordError(flowErr)
			span.SetStatus(codes.Error, flowErr.Error())
		}
		flowOutcomeCounter.Add(flowCtx, 1, metric.WithAttributes(
			attribute.String("flow", "address_delete"),
			attribute.String("outcome", outcome),
		))
		elapsed := time.Since(start).Seconds()
		flowDurationHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_delete"),
		))
		flowFreshnessHistogram.Record(flowCtx, elapsed, metric.WithAttributes(
			attribute.String("flow", "address_delete"),
		))
	}()

	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")
	addressId := ctx.Params("addressId")

	request := &model.DeleteAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
		ID:        addressId,
	}

	if err := c.UseCase.Delete(flowCtx, request); err != nil {
		c.Log.WithError(err).Error("failed to delete address")
		flowErr = err
		return flowErr
	}

	return ctx.JSON(model.WebResponse[bool]{Data: true})
}
