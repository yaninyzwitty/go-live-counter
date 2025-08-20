package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/joho/godotenv"
	"github.com/yaninyzwitty/go-live-counter/gen/like/v1/likev1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
	db "github.com/yaninyzwitty/go-live-counter/internal/database"
	likeHandler "github.com/yaninyzwitty/go-live-counter/internal/handlers"
	"github.com/yaninyzwitty/go-live-counter/internal/queue"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	// Structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load config
	var cfg config.Config
	if err := cfg.LoadConfig("config.yaml"); err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if err := godotenv.Load(); err != nil {
		slog.Warn("failed to load .env", "error", err)
	}

	// load CockroachDB password from env or config, fail fast if missing
	cockroachDBPassword := os.Getenv("COCKROACH_PASSWORD")

	if cockroachDBPassword == "" {
		slog.Error("missing CockroachDB password: set COCKROACH_PASSWORD or config.database.password")
		os.Exit(1)
	}

	// configuration for cockroach db
	roachConfig := &db.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.Username,
		Password: cockroachDBPassword,
		Database: cfg.Database.Database,
		SSLMode:  "verify-full",
	}

	// setup new db connection
	roachConn, err := db.New(30, 1*time.Second, roachConfig)
	if err != nil {
		slog.Error("failed to connect to CockroachDB", "error", err)
		os.Exit(1)
	}

	defer roachConn.Close()

	// create cocroach db pool
	cocroachPool := roachConn.Pool()

	tokenStr := os.Getenv("PULSAR_TOKEN")
	if tokenStr == "" {
		slog.Error("missing token string")
		os.Exit(1)
	}

	// pulsar config
	pulsarConfig := queue.PulsarConfig{
		ServiceURL: cfg.Queue.Url,
		TokenStr:   tokenStr,
	}

	pw, err := queue.NewPulsarWrapper(pulsarConfig)
	if err != nil {
		slog.Error("failed to create pulsar wrapper: ", "error", err)
	}
	defer pw.Close()

	// create consumer
	consumer, err := pw.CreateConsumer(cfg.Queue.TopicName, "test-subscription", pulsar.Shared)
	if err != nil {
		slog.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// register all queries and mutations
	roachQueries := repository.New(cocroachPool)

	// add the handler
	likeHandler := likeHandler.NewLike(roachQueries, cocroachPool, consumer)

	// Create HTTP mux and register services
	mux := http.NewServeMux()
	likePath, likeServiceHandler := likev1connect.NewLikeServiceHandler(likeHandler)
	mux.Handle(likePath, likeServiceHandler)

	// Create HTTP server with h2c (HTTP/2 without TLS for local/dev)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.LikeService.Port),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Graceful shutdown setup
	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		logger.Info("Shutdown signal received")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", "error", err)
		} else {
			logger.Info("Server shutdown gracefully")
		}
	}()

	// Start server
	logger.Info("Starting ConnectRPC server", "address", server.Addr, "pid", os.Getpid())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
