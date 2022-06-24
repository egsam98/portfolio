package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/egsam98/portfolio/amqp"
	"github.com/egsam98/portfolio/api/mq"
	"github.com/egsam98/portfolio/api/rest"
	_ "github.com/egsam98/portfolio/api/rest/docs"
	"github.com/egsam98/portfolio/config"
	"github.com/egsam98/portfolio/domain/gateways"
	"github.com/egsam98/portfolio/domain/portfolio"
	"github.com/egsam98/portfolio/pg"
	"github.com/go-redis/redis/v9"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

// TODO:
// 1. Graceful shutdown with context for portfolios
// 2. Logs with namespaces

const (
	ConfigPath      = "config.yaml"
	HealthPort      = 9090
	RESTPort        = 8080
	ShutdownTimeout = 15 * time.Second
)

var version string // "-X main.version=${VERSION}"

func main() {
	if err := run(); err != nil {
		log.Error().Stack().Err(err).Msg("Failed to start application")
	}
}

func run() error {
	// Load config
	cfg, err := config.Load(ConfigPath)
	if err != nil {
		return err
	}

	// Logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	var writer io.Writer = os.Stdout
	if cfg.Log.Pretty {
		writer = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}
	logLvl := zerolog.DebugLevel
	if lvl, err := zerolog.ParseLevel(cfg.Log.Level); err == nil {
		logLvl = lvl
	}
	log.Logger = zerolog.New(writer).
		With().
		Caller().
		Timestamp().
		Logger().
		Level(logLvl)

	log.Info().Interface("config", cfg).Msg("YAML-config has been successfully loaded")

	secret, err := os.ReadFile(cfg.JWTSecretPath)
	if err != nil {
		return errors.Wrap(err, "failed to read secret key")
	}

	dbCfg := &pg.Config{
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Host:            cfg.DB.Host,
		Name:            cfg.DB.Name,
		DisableTLS:      cfg.DB.DisableTLS,
		CertPath:        cfg.DB.CertPath,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxConnLifeTime: time.Minute * time.Duration(cfg.DB.MaxConnLifeTimeMins),
	}
	if cfg.DB.Log {
		dbCfg.Logger = zerolog.New(writer).
			With().
			Timestamp().
			Logger().
			Level(zerolog.DebugLevel)
	}

	db, err := pg.NewDB(dbCfg)
	if err != nil {
		return err
	}
	defer func() {
		log.Info().Msg("Closing PostgreSQL...")
		db.Close()
	}()

	// RabbitMQ and publishers
	rabbit := amqp.NewConnection(cfg.RabbitMQ.URI, cfg.ServerName)
	if err := rabbit.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := rabbit.Shutdown(); err != nil {
			log.Error().Stack().Err(err).Msg("Failed to close RabbitMQ connection")
		}
	}()

	rabbitPool := amqp.NewPool(rabbit, cfg.RabbitMQ.ChannelPoolSize)
	if err := rabbitPool.Start(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gwsMngr := gateways.NewManager()
	defer gwsMngr.Stop()
	gwsMngr.Start(ctx)

	triggerEventPublisher := mq.TriggerEventPublisher(rabbitPool)

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return errors.Wrap(err, "failed to ping redis")
	}

	defer func() {
		log.Info().Msg("Closing Redis...")
		if err := rdb.Close(); err != nil {
			log.Err(err).Msg("Failed to close Redis")
		}
	}()

	pm := portfolio.NewManager(db, rdb, gwsMngr, triggerEventPublisher)
	if err := pm.Start(ctx); err != nil {
		return err
	}
	defer pm.Close()

	defer func() {
		log.Info().Msgf("Closing RabbitMQ channels pool...")
		rabbitPool.Close()
	}()

	// MQ consumers
	mq.NewEventConsumer(cfg.ServerName, rabbitPool, pm).Start(ctx)

	// Health server
	httpErrs := make(chan error)
	go func() {
		log.Info().Msgf("Starting Health HTTP server on port %d...", HealthPort)
		if err := http.ListenAndServe(
			fmt.Sprintf("0.0.0.0:%d", HealthPort),
			rest.InitHealthHandler(version, db, rabbit, rabbitPool, gwsMngr),
		); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrs <- err
		}
	}()

	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", RESTPort),
		Handler: rest.InitHandler(secret, pm),
	}
	go func() {
		log.Info().Msgf("Starting HTTP server on port %d...", RESTPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			httpErrs <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGABRT,
		syscall.SIGTERM,
		syscall.SIGTRAP,
	)

	select {
	case err := <-httpErrs:
		return errors.Wrap(err, "server error")
	case sig := <-signals:
		log.Info().Msgf("Received signal: %s", sig)
		cancel()
		log.Info().Msg("Start shutdown")

		ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()

		// Give outstanding requests a deadline for completion.
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
			return errors.Wrap(err, "could not stop server gracefully")
		}

		log.Info().Msg("Shutdown done")
	}

	return nil
}
