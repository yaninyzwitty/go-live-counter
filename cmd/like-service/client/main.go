package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/like/v1/likev1connect"
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
	likeServiceUrl := fmt.Sprintf("http://localhost:%d", cfg.LikeService.Port)

	likeServiceClient := likev1connect.NewLikeServiceClient(
		httpClient,
		likeServiceUrl,
	)

	req := connect.NewRequest(&likev1.CreateLikeRequest{
		PostId: "5cd0cb60-4d98-479a-b226-c7130362fa8b",
		UserId: "d28471b8-e006-476f-b2c9-88f02dfbc4ae",
	})

	res, err := likeServiceClient.CreateLike(context.TODO(), req)

	if err != nil {
		slog.Error("failed to like", "error", err)
		os.Exit(1)
	}

	slog.Info("res", "liked-post-id", res.Msg.Like.PostId)

}
