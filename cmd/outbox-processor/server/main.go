package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	outboxv1 "github.com/yaninyzwitty/go-live-counter/gen/outbox/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/outbox/v1/outboxv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
	db "github.com/yaninyzwitty/go-live-counter/internal/database"
	"github.com/yaninyzwitty/go-live-counter/internal/handlers"
	"github.com/yaninyzwitty/go-live-counter/internal/queue"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

const (
	NUM_WORKERS = 5
)

var logger *slog.Logger

func main() {
	// --- Structured logging ---
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// --- Load config ---
	var cfg config.Config
	if err := cfg.LoadConfig("config.yaml"); err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	if err := godotenv.Load(); err != nil {
		logger.Warn("failed to load .env", "error", err)
	}

	// --- CockroachDB setup ---
	cockroachDBPassword := os.Getenv("COCKROACH_PASSWORD")
	if cockroachDBPassword == "" {
		logger.Error("missing CockroachDB password: set COCKROACH_PASSWORD or config.database.password")
		os.Exit(1)
	}

	roachConfig := &db.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.Username,
		Password: cockroachDBPassword,
		Database: cfg.Database.Database,
		SSLMode:  "verify-full",
	}

	roachConn, err := db.New(30, 1*time.Second, roachConfig)
	if err != nil {
		logger.Error("failed to connect to CockroachDB", "error", err)
		os.Exit(1)
	}
	defer roachConn.Close()

	cockroachPool := roachConn.Pool()
	roachQueries := repository.New(cockroachPool)

	// --- Pulsar setup ---
	tokenStr := os.Getenv("PULSAR_TOKEN")
	if tokenStr == "" {
		logger.Error("missing token string")
		os.Exit(1)
	}

	pulsarConfig := queue.PulsarConfig{
		ServiceURL: cfg.Queue.Url,
		TokenStr:   tokenStr,
	}

	pw, err := queue.NewPulsarWrapper(pulsarConfig)
	if err != nil {
		logger.Error("failed to create pulsar wrapper", "error", err)
		os.Exit(1)
	}
	defer pw.Close()

	producer, err := pw.CreateProducer(cfg.Queue.TopicName)
	if err != nil {
		logger.Error("failed to create producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	// --- Handlers ---
	outboxHandler := handlers.NewOutbox(roachQueries, producer)

	// --- HTTP mux / server ---
	mux := http.NewServeMux()
	outboxPath, outboxServiceHandler := outboxv1connect.NewProcessorServiceHandler(outboxHandler)
	mux.Handle(outboxPath, outboxServiceHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Outbox.Port),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// --- Context & shutdown handling ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wg := &sync.WaitGroup{}

	// Worker pool
	wg.Add(1)
	go func() {
		defer wg.Done()
		startPool(ctx, outboxHandler)
	}()

	// HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting ConnectRPC server", "address", server.Addr, "pid", os.Getpid())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed", "error", err)
			stop() // trigger shutdown
		}
	}()

	// --- Wait for shutdown ---
	<-ctx.Done()
	logger.Info("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	} else {
		logger.Info("Server shutdown gracefully")
	}

	wg.Wait()
	logger.Info("Shutdown complete")
}

// --- Worker pool ---
func startPool(ctx context.Context, outboxHandler *handlers.OutboxStoreServiceHandler) {
	for i := 0; i < 1; i++ {
		go func(workerID int) {
			logger.Info("worker started", "id", workerID)
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					logger.Info("worker stopped", "id", workerID)
					return
				case <-ticker.C:
					if err := processEvents(ctx, outboxHandler); err != nil {
						logger.Error("Worker failed", "id", workerID, "error", err)
					}
				}
			}
		}(i)
	}
}

// --- Event processor ---
func processEvents(ctx context.Context, outboxHandler *handlers.OutboxStoreServiceHandler) error {
	req := connect.NewRequest(&outboxv1.ProcessOutboxMessageRequest{
		Published: false,
	})

	resp, err := outboxHandler.ProcessOutboxMessage(ctx, req)
	if err != nil {
		return fmt.Errorf("error processing events: %w", err)
	}
	if resp != nil && resp.Msg != nil && resp.Msg.ProcessedCount > 0 {
		logger.Info("processed events", "events", resp.Msg.ProcessedCount)
	}
	return nil
}
