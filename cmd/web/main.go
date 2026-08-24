package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"time"

	"github.com/gofiber/contrib/otelfiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	producer := config.NewKafkaProducer(viperConfig, log)

	otelCtx := context.Background()

	otelEndpoint := viperConfig.GetString("otel.exporter.otlp.endpoint")

	otelRes, resErr := resource.New(otelCtx,
		resource.WithAttributes(
			semconv.ServiceName("golang-clean-architecture"),
		),
	)
	if resErr != nil {
		log.Fatalf("failed to create otel resource: %v", resErr)
	}

	traceExporterOpts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure()}
	if otelEndpoint != "" {
		traceExporterOpts = append(traceExporterOpts, otlptracegrpc.WithEndpoint(otelEndpoint))
	}
	traceExporter, traceExpErr := otlptracegrpc.New(otelCtx, traceExporterOpts...)
	if traceExpErr != nil {
		log.Fatalf("failed to create otlp trace exporter: %v", traceExpErr)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(otelRes),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporterOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithInsecure()}
	if otelEndpoint != "" {
		metricExporterOpts = append(metricExporterOpts, otlpmetricgrpc.WithEndpoint(otelEndpoint))
	}
	metricExporter, metricExpErr := otlpmetricgrpc.New(otelCtx, metricExporterOpts...)
	if metricExpErr != nil {
		log.Fatalf("failed to create otlp metric exporter: %v", metricExpErr)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(otelRes),
	)
	otel.SetMeterProvider(meterProvider)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := tracerProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Errorf("failed to shutdown otel tracer provider: %v", shutdownErr)
		}
		if shutdownErr := meterProvider.Shutdown(shutdownCtx); shutdownErr != nil {
			log.Errorf("failed to shutdown otel meter provider: %v", shutdownErr)
		}
	}()

	app.Use(otelfiber.Middleware())

	config.Bootstrap(&config.BootstrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   viperConfig,
		Producer: producer,
	})

	webPort := viperConfig.GetInt("web.port")
	err := app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
