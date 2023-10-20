package postgres

import (
	"database/sql"
	"embed"
	"github.com/catalystsquad/app-utils-go/env"
	"github.com/catalystsquad/app-utils-go/errorutils"
	"github.com/catalystsquad/app-utils-go/logging"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

type PostgresStorage struct{}

const migrationsDirectory = "migrations"

var uri = env.GetEnvOrDefault("POSTGRES_URI", "postgresql://postgres:postgres@localhost:5432?sslmode=disable")
var gormLogLevel = env.GetEnvOrDefault("POSTGRES_LOG_LEVEL", "error")
var ignoreRecordNotFoundError = env.GetEnvAsBoolOrDefault("POSTGRES_LOG_IGNORE_NOT_FOUND", "true")
var parameterizedQueries = env.GetEnvAsBoolOrDefault("POSTGRES_LOG_PARAMETERIZED_QUERIES", "true")
var slowQueryThreshold = env.GetEnvAsDurationOrDefault("POSTGRES_LOG_SLOW_QUERY_THRESHOLD", "1s")
var maxIdleConns = env.GetEnvAsIntOrDefault("POSTGRES_MAX_IDLE_CONNS", "10")
var maxOpenConns = env.GetEnvAsIntOrDefault("POSTGRES_MAX_OPEN_CONNS", "100")
var connMaxLifetime = env.GetEnvAsDurationOrDefault("POSTGRES_CONN_MAX_LIFETIME", "1h")
var connMaxIdleTime = env.GetEnvAsDurationOrDefault("POSTGRES_CONN_MAX_IDLE_TIME", "10m")
var db *gorm.DB

//go:embed migrations/*.sql
var migrations embed.FS

func (p PostgresStorage) Initialize() (shutdown func(), err error) {
	config := &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
			logger.Config{
				SlowThreshold:             slowQueryThreshold,
				ParameterizedQueries:      parameterizedQueries,
				LogLevel:                  getGormLogLevel(),         // Log level
				IgnoreRecordNotFoundError: ignoreRecordNotFoundError, // Ignore ErrRecordNotFound error for logger
				Colorful:                  true,                      // Disable color
			},
		),
	}
	if db, err = gorm.Open(postgres.Open(uri), config); err != nil {
		return nil, err
	}
	var sqlDb *sql.DB
	if sqlDb, err = db.DB(); err != nil {
		return nil, err
	}
	sqlDb.SetMaxIdleConns(maxIdleConns)
	sqlDb.SetMaxOpenConns(maxOpenConns)
	sqlDb.SetConnMaxLifetime(connMaxLifetime)
	sqlDb.SetConnMaxIdleTime(connMaxIdleTime)
	logging.Log.Info("connected to postgres")
	//err = db.AutoMigrate(&katanov1.AgentGormModel{})
	//if err != nil {
	//	panic(err)
	//}
	runMigrations(migrationsDirectory)
	return nil, nil
}

func getGormLogLevel() logger.LogLevel {
	switch gormLogLevel {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Error
	}
}

func (p PostgresStorage) Ready() bool {
	sqlDb, err := db.DB()
	if err != nil {
		return false
	}
	return sqlDb.Ping() == nil
}

func runMigrations(directory string) {
	var err error
	var db *sql.DB
	var success bool
	goose.SetBaseFS(migrations)
	for !success {
		if db, err = sql.Open("postgres", uri); err != nil {
			errorutils.LogOnErr(nil, "error opening connection to database during sql migrations", err)
			time.Sleep(5 * time.Second)
			continue
		}
		defer db.Close()
		if err = goose.Up(db, directory); err != nil {
			errorutils.LogOnErr(nil, "error running migrations", err)
			time.Sleep(5 * time.Second)
			continue
		}
		success = true
	}
}
