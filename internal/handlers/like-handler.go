package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	postv1 "github.com/yaninyzwitty/go-live-counter/gen/post/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/post/v1/postv1connect"
	userv1 "github.com/yaninyzwitty/go-live-counter/gen/user/v1"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// LikeStoreServiceHandler implements the UserService API.

type LikeStoreServiceHandler struct {
	postv1connect.UnimplementedPostServiceHandler
	Queries *repository.Queries
}

// CREATE new instance of user handler
func NewLike(queries *repository.Queries) *LikeStoreServiceHandler {
	return &LikeStoreServiceHandler{
		Queries: queries,
	}
}

func (h *LikeStoreServiceHandler) CreateLike(ctx context.Context, req *connect.Request[likev1.CreateLikeRequest]) (*connect.Response[likev1.CreateLikeResponse], error) {
	userId := strings.TrimSpace(req.Msg.UserId)
	postId := strings.TrimSpace(req.Msg.PostId)

	if userId == "" || postId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("userId and postId are required"))
	}

	// Insert into DB via sqlc
	uid, err := uuid.Parse(userId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid userId: %w", err))
	}
	pid, err := uuid.Parse(postId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid postId: %w", err))
	}

	like, err := h.Queries.InsertLike(ctx, repository.InsertLikeParams{
		UserID: uid,
		PostID: pid,
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique violation
				return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already liked this post"))
			case "23503": // foreign key violation
				return nil, connect.NewError(connect.CodeNotFound, errors.New("user or post not found"))
			}
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to insert like: %w", err))
	}

	// Build response
	res := &likev1.CreateLikeResponse{
		Like: &likev1.Like{
			Post: &postv1.Post{
				Id: like.PostID.String()},
			User: &userv1.User{
				Id: like.UserID.String(),
			},
			CreatedAt: timestamppb.New(like.CreatedAt),
		},
	}

	return connect.NewResponse(res), nil

}
