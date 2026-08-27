package config

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitOTel builds the OpenTelemetry TracerProvider and MeterProvider for this
// process, exporting via OTLP/gRPC to an endpoint driven by the standard
// OTEL_EXPORTER_OTLP_ENDPOINT environment variable, and registers both as the
// global providers. It returns a shutdown function that MUST be deferred by
// the caller so buffered spans/metrics are flushed on exit.
//
// Go has no bytecode agent, so this application always owns SDK setup; this
// function must be called exactly once, from main().
func InitOTel(ctx context.Context, log *logrus.Logger, serviceName string) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	if log != nil {
		log.Infof("OpenTelemetry initialized for service %q", serviceName)
	}

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	return shutdown, nil
}
