package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
	"time"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)

	shutdownTelemetry := config.InitTelemetry(context.Background(), "golang-clean-architecture-web", log)
	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			log.WithError(err).Error("failed to shutdown telemetry")
		}
	}()

	db := config.NewDatabase(viperConfig, log)
	validate := config.NewValidator(viperConfig)
	app := config.NewFiber(viperConfig)
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
