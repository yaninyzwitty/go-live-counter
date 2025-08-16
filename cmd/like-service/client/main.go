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
		PostId: "403ba5f1-b7d9-429d-8a14-5d840ca1e8dd",
		UserId: "3be6c9ba-d2f4-4fb5-8f32-ba7dd1d00466",
	})

	res, err := likeServiceClient.CreateLike(context.TODO(), req)

	if err != nil {
		slog.Error("failed to like", "error", err)
		os.Exit(1)
	}

	slog.Info("res", "liked-post-id", res.Msg.Like.Post.Id)

}
