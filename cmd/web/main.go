package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"golang-clean-architecture/internal/telemetry"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)
	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
	producer := config.NewKafkaProducer(viperConfig, log)

	otelShutdown, err := telemetry.Setup(context.Background(), "golang-clean-architecture")
	if err != nil {
		log.Fatalf("Failed to set up OpenTelemetry: %v", err)
	}
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			log.Errorf("Failed to shut down OpenTelemetry: %v", shutdownErr)
		}
	}()

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
