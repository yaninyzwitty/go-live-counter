package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"connectrpc.com/connect"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/like/v1/likev1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
)

var (
	clients   = make(map[chan string]bool)
	clientsMu sync.Mutex
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

	// Setup client
	httpClient := http.DefaultClient
	likeServiceURL := fmt.Sprintf("http://localhost:%d", cfg.LikeService.Port)
	likeClient := likev1connect.NewLikeServiceClient(httpClient, likeServiceURL)

	// Prepare context with cancel (so Ctrl+C stops the stream)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	req := connect.NewRequest(&likev1.StreamLikesRequest{
		PostId: "5cd0cb60-4d98-479a-b226-c7130362fa8b",
	})

	// Start background goroutine to stream from LikeService
	go func() {
		stream, err := likeClient.StreamLikes(ctx, req)
		if err != nil {
			slog.Error("failed to start stream", "error", err)
			os.Exit(1)
		}

		for stream.Receive() {
			update := stream.Msg()
			msg := fmt.Sprintf(`{"post_id":"%s","total_likes":%s,"liked_at":"%s"}`,
				update.PostId,
				strconv.Itoa(int(update.TotalLikes)),
				update.LikedAt.AsTime().Format("2006-01-02T15:04:05Z07:00"),
			)

			// Log + forward to SSE clients
			slog.Info("like update received", "msg", msg)
			broadcast(msg)
		}

		if err := stream.Err(); err != nil {
			slog.Error("stream ended with error", "error", err)
		}
	}()
	// SSE endpoint
	http.HandleFunc("/events", handleEvents)

	// Run HTTP server for browsers
	logger.Info("SSE Gateway running at http://localhost:8080/events")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.Error("server failed", "error", err)
	}

}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	messageChan := make(chan string, 10)
	clientsMu.Lock()
	clients[messageChan] = true
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clients, messageChan)
		clientsMu.Unlock()
		close(messageChan)
	}()

	// stream messages to the client
	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}

}

// broadcast pushes messages to all connected SSE clients
func broadcast(msg string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for ch := range clients {
		select {
		case ch <- msg:
		default:
			// drop if channel is full
		}
	}
}
