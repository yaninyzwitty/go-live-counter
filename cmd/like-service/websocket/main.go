package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/yaninyzwitty/go-live-counter/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Load config
	var cfg config.Config
	if err := cfg.LoadConfig("config.yaml"); err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	websocketAddr := fmt.Sprintf(":%d", cfg.Websocket.Port)

	// create a listener
	lis, err := net.Listen("tcp", websocketAddr)
	if err != nil {
		slog.Error("failed to listen ", "error", err)
		os.Exit(1)
	}

	slog.Info("listening", "endpoint", "ws://"+lis.Addr().String())

	mux := http.NewServeMux()
	mux.Handle("/ws", &gatewayServer{cfg: &cfg, logger: logger})
	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.Serve(lis); err != nil {
		logger.Error("websocket gateway stopped", "error", err)
	}
}
