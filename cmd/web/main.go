package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"golang-clean-architecture/internal/delivery/http/middleware"

	"github.com/gofiber/contrib/otelfiber/v2"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	producer := config.NewKafkaProducer(viperConfig, log)

	shutdownTelemetry, err := config.InitTelemetry(context.Background())
	if err != nil {
		log.Fatalf("Failed to initialize telemetry: %v", err)
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Warnf("Failed to shutdown telemetry: %v", err)
		}
	}()

	httpInstruments, err := middleware.NewHTTPInstruments()
	if err != nil {
		log.Fatalf("Failed to create http instruments: %v", err)
	}

	app.Use(otelfiber.Middleware())
	app.Use(middleware.Telemetry(httpInstruments))

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
