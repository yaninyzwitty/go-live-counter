package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/post/v1/postv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LikeStoreServiceHandler implements the UserService API.

const (
	USER_LIKE_EVENT_TYPE = "USER_LIKE_EVENT_TYPE"
)

type LikeStoreServiceHandler struct {
	postv1connect.UnimplementedPostServiceHandler
	Queries  *repository.Queries
	Db       *pgxpool.Pool
	Consumer pulsar.Consumer
}

// CREATE new instance of user handler
func NewLike(queries *repository.Queries, db *pgxpool.Pool, consumer pulsar.Consumer) *LikeStoreServiceHandler {
	return &LikeStoreServiceHandler{
		Queries:  queries,
		Db:       db,
		Consumer: consumer,
	}
}

func (h *LikeStoreServiceHandler) CreateLike(ctx context.Context, req *connect.Request[likev1.CreateLikeRequest]) (*connect.Response[likev1.CreateLikeResponse], error) {
	userId := strings.TrimSpace(req.Msg.UserId)
	postId := strings.TrimSpace(req.Msg.PostId)

	if userId == "" || postId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("userId and postId are required"),
		)
	}

	// Start transaction
	tx, err := h.Db.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to start transaction: %w", err),
		)
	}
	defer tx.Rollback(ctx)

	// Wrap sqlc with transaction
	qtx := h.Queries.WithTx(tx)

	// Build LikeEvent proto
	likeEvent := &likev1.Like{
		PostId:    postId,
		UserId:    userId,
		CreatedAt: timestamppb.Now(),
	}

	// Marshal event payload
	payload, err := protojson.Marshal(likeEvent)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to marshal payload: %w", err),
		)
	}

	// Insert event into outbox
	if _, err := qtx.InsertPayloadEvent(ctx, repository.InsertPayloadEventParams{
		EventType: USER_LIKE_EVENT_TYPE,
		Payload:   payload,
	}); err != nil {
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to insert event into outbox: %w", err),
		)
	}

	// Parse UUIDs
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid userId: %w", err),
		)
	}
	pid, err := uuid.Parse(postId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid postId: %w", err),
		)
	}

	// Insert the Like record
	like, err := qtx.InsertLike(ctx, repository.InsertLikeParams{
		UserID: uid,
		PostID: pid,
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique violation
				return nil, connect.NewError(connect.CodeAlreadyExists,
					errors.New("user already liked this post"))
			case "23503": // foreign key violation
				return nil, connect.NewError(connect.CodeNotFound,
					errors.New("user or post not found"))
			}
		}
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to insert like: %w", err),
		)
	}

	// Commit transaction only if both inserts succeeded
	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(
			connect.CodeInternal,
			fmt.Errorf("failed to commit transaction: %w", err),
		)
	}

	// Build response
	res := &likev1.CreateLikeResponse{
		Like: &likev1.Like{
			PostId:    postId,
			UserId:    userId,
			CreatedAt: timestamppb.New(like.CreatedAt),
		},
	}

	return connect.NewResponse(res), nil

}

func (h *LikeStoreServiceHandler) StreamLikes(
	ctx context.Context,
	req *connect.Request[likev1.StreamLikesRequest],
	stream *connect.ServerStream[likev1.LikeUpdate],
) error {
	postIdStr := req.Msg.PostId
	if postIdStr == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("post id is required"))
	}

	postID, err := uuid.Parse(postIdStr)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid post id: %w", err))
	}

	// Get the current number of likes from DB
	totalLikes, err := h.Queries.CountLikesByPost(ctx, postID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count total likes: %w", err))
	}

	for {
		msg, err := h.Consumer.Receive(ctx)
		if err != nil {
			// Exit gracefully if the client disconnected
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to receive message: %w", err))
		}

		eventType := msg.Properties()["event_type"]

		var like likev1.Like
		if err := protojson.Unmarshal(msg.Payload(), &like); err != nil {
			slog.Error("failed to unmarshal like event", "err", err)
			// ack anyway to avoid poison-message loop
			_ = h.Consumer.Ack(msg)
			continue
		}

		if like.GetPostId() == postIdStr {
			switch eventType {
			case USER_LIKE_EVENT_TYPE:
				totalLikes++
			// case USER_UNLIKE_EVENT_TYPE:
			// 	if totalLikes > 0 {
			// 		totalLikes--
			// 	}
			default:
				_ = h.Consumer.Ack(msg)
				continue
			}

			update := &likev1.LikeUpdate{
				PostId:     like.GetPostId(),
				TotalLikes: totalLikes,
				LikedAt:    like.CreatedAt,
			}

			if err := stream.Send(update); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to send stream: %w", err))
			}
		}

		// Ack message after processing
		if ackErr := h.Consumer.Ack(msg); ackErr != nil {
			slog.Warn("failed to ack message", "err", ackErr)
		}
	}
}
