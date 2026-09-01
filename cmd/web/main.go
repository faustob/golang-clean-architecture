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

	ctx := context.Background()
	shutdownTelemetry, err := telemetry.InitProviders(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if shutdownErr := shutdownTelemetry(ctx); shutdownErr != nil {
			log.WithError(shutdownErr).Error("failed to shutdown OpenTelemetry providers")
		}
	}()

	app.Use(otelfiber.Middleware())
	app.Use(telemetry.RequestOutcomeMiddleware())

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
