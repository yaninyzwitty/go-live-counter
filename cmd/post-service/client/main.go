package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	postv1 "github.com/yaninyzwitty/go-live-counter/gen/post/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/post/v1/postv1connect"
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
	postServiceUrl := fmt.Sprintf("http://localhost:%d", cfg.PostService.Port)

	postServiceClient := postv1connect.NewPostServiceClient(
		httpClient,
		postServiceUrl,
	)

	req := connect.NewRequest(&postv1.CreatePostRequest{
		UserId:  "e3c21797-fff2-4e4c-9dbe-cd1bc461c643",
		Content: `Shoutout to my wrinkly hand `,
	})

	res, err := postServiceClient.CreatePost(context.TODO(), req)

	if err != nil {
		slog.Error("failed to create user", "error", err)
		os.Exit(1)
	}

	slog.Info("res", "created-at", res.Msg.Post.Id)

}
