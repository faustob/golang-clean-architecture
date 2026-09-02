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
	otelShutdown := config.NewOpenTelemetry(viperConfig, log)
	defer otelShutdown(context.Background())
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	app.Use(otelfiber.Middleware())
	app.Use(middleware.NewTelemetryMiddleware())
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
	err := app.Listen(fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
