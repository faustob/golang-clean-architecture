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

	otelCtx := context.Background()
	shutdownOTel, err := config.InitOTel(otelCtx, log, "golang-clean-architecture")
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdownOTel(context.Background()); err != nil {
			log.Errorf("Failed to shutdown OpenTelemetry: %v", err)
		}
	}()

	if err := telemetry.InitInstruments(); err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry instruments: %v", err)
	}

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
	err = app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
