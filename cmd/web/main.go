package main

import (
	"context"
	"fmt"
	"golang-clean-architecture/internal/config"
)

func main() {
	viperConfig := config.NewViper()
	log := config.NewLogger(viperConfig)

	// Register the global OTel MeterProvider before any instrumented code
	// (e.g. the otelsql-wrapped database connection) runs.
	meterProvider := config.NewMeterProvider(log)
	defer func() {
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			log.Errorf("failed to shutdown otel meter provider: %v", err)
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
