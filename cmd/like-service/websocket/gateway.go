package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/like/v1/likev1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/config"
	"golang.org/x/net/http2"
)

// gatewayServer bridges WebSocket ↔ gRPC streaming.
type gatewayServer struct {
	cfg    *config.Config
	logger *slog.Logger
}

func (s *gatewayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.logger.Error("failed to accept handshake", "error", err)
		return
	}

	defer c.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		postID = "5cd0cb60-4d98-479a-b226-c7130362fa8b"
	}

	req := connect.NewRequest(&likev1.StreamLikesRequest{
		PostId: postID,
	})
	// start stream with h2c support
	tr := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	httpClient := &http.Client{Transport: tr}
	likeServiceURL := fmt.Sprintf("http://localhost:%d", s.cfg.LikeService.Port)
	likeClient := likev1connect.NewLikeServiceClient(httpClient, likeServiceURL)

	stream, err := likeClient.StreamLikes(ctx, req)
	if err != nil {
		s.logger.Error("failed to open grpc likes stream", "error", err)
		c.Close(websocket.StatusInternalError, "failed to connect upstream")
		return
	}

	// Watch client disconnects by reading frames
	go func() {
		for {
			if _, _, err := c.Read(ctx); err != nil {
				cancel()
				return
			}
		}
	}()

	// Relay upstream messages to websocket client
	for stream.Receive() {
		update := stream.Msg()
		slog.Info("like update received",
			"post_id", update.PostId,
			"total_likes", update.TotalLikes,
			"liked_at", update.LikedAt.AsTime(),
		)
		payload := struct {
			PostID     string `json:"post_id"`
			TotalLikes int64  `json:"total_likes"`
			LikedAt    string `json:"liked_at"`
		}{
			PostID:     update.PostId,
			TotalLikes: update.TotalLikes,
			LikedAt:    update.LikedAt.AsTime().UTC().Format(time.RFC3339),
		}

		b, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error("failed to marshal update", "error", err)
			continue
		}

		if err := c.Write(ctx, websocket.MessageText, b); err != nil {
			s.logger.Error("failed to write to websocket", "error", err)
			cancel()
			break
		}
	}

	if err := stream.Err(); err != nil {
		s.logger.Info("upstream stream closed", "error", err)
	}
}
