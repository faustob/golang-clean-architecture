package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"

	otelfiber "github.com/gofiber/contrib/otelfiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// initOtelSDK builds and globally registers the OpenTelemetry tracer and
// meter providers, exporting via OTLP/gRPC. The endpoint is env-driven via
// the standard OTEL_EXPORTER_OTLP_ENDPOINT (and related) environment
// variables. It returns a shutdown function that flushes buffered telemetry.
func initOtelSDK(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("golang-clean-architecture"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build otel resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create otlp metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http.server.request.duration"},
			sdkmetric.Stream{
				Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10},
				},
			},
		)),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	_, err = otel.Meter("golang-clean-architecture").Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http.server.request.duration histogram: %w", err)
	}

	return func(shutdownCtx context.Context) error {
		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return meterProvider.Shutdown(shutdownCtx)
	}, nil
}

func main() {
	otelCtx := context.Background()
	shutdownOtel, err := initOtelSDK(otelCtx)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize opentelemetry: %v", err))
	}
	defer func() {
		if err := shutdownOtel(context.Background()); err != nil {
			fmt.Printf("failed to shutdown opentelemetry: %v\n", err)
		}
	}()

	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	app.Use(otelfiber.Middleware())
	producer := config.NewKafkaProducer(viperConfig, log)

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})

	webPort := viperConfig.GetInt("web.port")
	err = app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
