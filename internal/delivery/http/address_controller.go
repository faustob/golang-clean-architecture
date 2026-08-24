package http

import (
	"fmt"
	"time"

	"golang-clean-architecture/internal/delivery/http/middleware"
	"golang-clean-architecture/internal/model"
	"golang-clean-architecture/internal/usecase"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var httpServerRequestDuration metric.Float64Histogram

func init() {
	hist, err := otel.Meter("golang-clean-architecture").Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
	)
	if err == nil {
		httpServerRequestDuration = hist
	}
}

func recordHTTPServerRequestDuration(ctx *fiber.Ctx, start time.Time) {
	if httpServerRequestDuration == nil {
		return
	}
	elapsed := time.Since(start).Seconds()
	httpServerRequestDuration.Record(ctx.UserContext(), elapsed,
		metric.WithAttributes(
			attribute.String("http.request.method", ctx.Method()),
			attribute.String("http.route", ctx.Route().Path),
			attribute.Int("http.response.status_code", ctx.Response().StatusCode()),
		),
	)
}

// slowRequestThreshold is the P99 latency budget; handler durations exceeding
// this are annotated with a span event for triage.
const slowRequestThreshold = 300 * time.Millisecond

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
	span := trace.SpanFromContext(ctx.UserContext())
	auth := middleware.GetUser(ctx)

	request := new(model.CreateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return fiber.ErrBadRequest
	}

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")

	response, err := c.UseCase.Create(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to create address")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return err
	}

	if elapsed := time.Since(start); elapsed > slowRequestThreshold {
		span.AddEvent("slow_request", trace.WithAttributes(attribute.Float64("duration_ms", float64(elapsed.Milliseconds()))))
	}
	recordHTTPServerRequestDuration(ctx, start)

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) List(ctx *fiber.Ctx) error {
	start := time.Now()
	span := trace.SpanFromContext(ctx.UserContext())
	auth := middleware.GetUser(ctx)
	contactId := ctx.Params("contactId")

	request := &model.ListAddressRequest{
		UserId:    auth.ID,
		ContactId: contactId,
	}

	responses, err := c.UseCase.List(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to list addresses")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return err
	}

	if elapsed := time.Since(start); elapsed > slowRequestThreshold {
		span.AddEvent("slow_request", trace.WithAttributes(attribute.Float64("duration_ms", float64(elapsed.Milliseconds()))))
	}
	recordHTTPServerRequestDuration(ctx, start)

	return ctx.JSON(model.WebResponse[[]model.AddressResponse]{Data: responses})
}

func (c *AddressController) Get(ctx *fiber.Ctx) error {
	start := time.Now()
	span := trace.SpanFromContext(ctx.UserContext())
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
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return err
	}

	if elapsed := time.Since(start); elapsed > slowRequestThreshold {
		span.AddEvent("slow_request", trace.WithAttributes(attribute.Float64("duration_ms", float64(elapsed.Milliseconds()))))
	}
	recordHTTPServerRequestDuration(ctx, start)

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Update(ctx *fiber.Ctx) error {
	start := time.Now()
	span := trace.SpanFromContext(ctx.UserContext())
	auth := middleware.GetUser(ctx)

	request := new(model.UpdateAddressRequest)
	if err := ctx.BodyParser(request); err != nil {
		c.Log.WithError(err).Error("failed to parse request body")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return fiber.ErrBadRequest
	}

	request.UserId = auth.ID
	request.ContactId = ctx.Params("contactId")
	request.ID = ctx.Params("addressId")

	response, err := c.UseCase.Update(ctx.UserContext(), request)
	if err != nil {
		c.Log.WithError(err).Error("failed to update address")
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return err
	}

	if elapsed := time.Since(start); elapsed > slowRequestThreshold {
		span.AddEvent("slow_request", trace.WithAttributes(attribute.Float64("duration_ms", float64(elapsed.Milliseconds()))))
	}
	recordHTTPServerRequestDuration(ctx, start)

	return ctx.JSON(model.WebResponse[*model.AddressResponse]{Data: response})
}

func (c *AddressController) Delete(ctx *fiber.Ctx) error {
	start := time.Now()
	span := trace.SpanFromContext(ctx.UserContext())
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
		span.SetAttributes(attribute.String("error.type", fmt.Sprintf("%T", err)))
		span.RecordError(err)
		return err
	}

	if elapsed := time.Since(start); elapsed > slowRequestThreshold {
		span.AddEvent("slow_request", trace.WithAttributes(attribute.Float64("duration_ms", float64(elapsed.Milliseconds()))))
	}
	recordHTTPServerRequestDuration(ctx, start)

	return ctx.JSON(model.WebResponse[bool]{Data: true})
}
