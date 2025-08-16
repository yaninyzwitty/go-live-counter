package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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
	like, err := h.Queries.InsertLike(ctx, repository.InsertLikeParams{
		UserID: uuid.MustParse(userId),
		PostID: uuid.MustParse(postId),
	})

	if err != nil {
		// Handle unique violation gracefully (already liked)
		if strings.Contains(err.Error(), "unique") {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user already liked this post"))
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
