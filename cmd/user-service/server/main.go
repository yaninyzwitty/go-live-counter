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

	"github.com/yaninyzwitty/go-live-counter/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// userStoreServiceHandler implements the UserService API.
type userStoreServiceHandler struct {
	userv1connect.UnimplementedUserServiceHandler
}

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

	// Create HTTP mux and register services
	mux := http.NewServeMux()
	userPath, userHandler := userv1connect.NewUserServiceHandler(&userStoreServiceHandler{})
	mux.Handle(userPath, userHandler)

	// Create HTTP server with h2c (HTTP/2 without TLS for local/dev)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.UserService.Port),
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
