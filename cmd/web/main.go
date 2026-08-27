package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"golang-clean-architecture/internal/telemetry"

	"github.com/gofiber/contrib/otelfiber/v2"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	producer := config.NewKafkaProducer(viperConfig, log)

	telemetryCtx := context.Background()
	shutdownTelemetry, err := telemetry.InitTelemetry(telemetryCtx, "golang-clean-architecture")
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdownTelemetry(telemetryCtx); err != nil {
			log.WithError(err).Error("failed to shutdown telemetry")
		}
	}()

	app.Use(otelfiber.Middleware())
	app.Use(telemetry.RequestTelemetryMiddleware())

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
