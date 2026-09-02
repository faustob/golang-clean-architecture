package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"golang-clean-architecture/internal/delivery/http/middleware"
	"golang-clean-architecture/internal/telemetry"

	"github.com/gofiber/contrib/otelfiber/v2"
)

func main() {
	ctx := context.Background()
	shutdownTelemetry, err := telemetry.Setup(ctx, "golang-clean-architecture")
	if err != nil {
		panic(fmt.Sprintf("failed to set up telemetry: %v", err))
	}
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			fmt.Printf("failed to shutdown telemetry: %v\n", err)
		}
	}()

	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	app.Use(otelfiber.Middleware())
	app.Use(middleware.Telemetry())
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
