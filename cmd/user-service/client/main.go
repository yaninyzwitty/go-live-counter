package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	userv1 "github.com/yaninyzwitty/go-live-counter/gen/user/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
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

	// set up http client
	httpClient := http.DefaultClient
	userServiceUrl := fmt.Sprintf("http://localhost:%d", cfg.UserService.Port)

	userServiceClient := userv1connect.NewUserServiceClient(
		httpClient,
		userServiceUrl,
	)

	req := connect.NewRequest(&userv1.CreateUserRequest{
		Name:  "Ian Mwangi",
		Email: "ianmwa143@outlook.com",
	})

	res, err := userServiceClient.CreateUser(context.TODO(), req)

	if err != nil {
		slog.Error("failed to create user", "error", err)
		os.Exit(1)
	}

	slog.Info("res", "created-at", res.Msg.User.CreatedAt)

}
