package config

import (
	"fmt"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(viper *viper.Viper, log *logrus.Logger) *gorm.DB {
	username := viper.GetString("database.username")
	password := viper.GetString("database.password")
	host := viper.GetString("database.host")
	port := viper.GetInt("database.port")
	database := viper.GetString("database.name")
	idleConnection := viper.GetInt("database.pool.idle")
	maxConnection := viper.GetInt("database.pool.max")
	maxLifeTimeConnection := viper.GetInt("database.pool.lifetime")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)

	// Open the underlying *sql.DB through otelsql so every query GORM issues is
	// automatically instrumented with the semconv db.client.operation.duration histogram
	// (backing the DB query latency p95 SLI) and per-call error/status attributes
	// (backing the DB error rate SLI, derived by filtering db.client.operation.duration
	// by its error.type attribute) — no hand-written callbacks/metrics needed.
	sqlDB, err := otelsql.Open("mysql", dsn, otelsql.WithAttributes(
		attribute.String("db.system.name", "mysql"),
		attribute.String("db.namespace", database),
	))
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
		DSN:  dsn,
	}), &gorm.Config{
		Logger: logger.New(&logrusWriter{Logger: log}, logger.Config{
			SlowThreshold:             time.Second * 5,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			LogLevel:                  logger.Info,
		}),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection, err := db.DB()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	connection.SetMaxIdleConns(idleConnection)
	connection.SetMaxOpenConns(maxConnection)
	connection.SetConnMaxLifetime(time.Second * time.Duration(maxLifeTimeConnection))

	return db
}

type logrusWriter struct {
	Logger *logrus.Logger
}

func (l *logrusWriter) Printf(message string, args ...interface{}) {
	l.Logger.Tracef(message, args...)
}
