package handlers

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	postv1 "github.com/yaninyzwitty/go-live-counter/gen/post/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/post/v1/postv1connect"
	userv1 "github.com/yaninyzwitty/go-live-counter/gen/user/v1"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PostStoreServiceHandler implements the PostService API.

type PostStoreServiceHandler struct {
	postv1connect.UnimplementedPostServiceHandler
	Queries *repository.Queries
}

// CREATE new instance of post handler
func NewPost(queries *repository.Queries) *PostStoreServiceHandler {
	return &PostStoreServiceHandler{
		Queries: queries,
	}
}

func (h *PostStoreServiceHandler) CreatePost(ctx context.Context, req *connect.Request[postv1.CreatePostRequest]) (*connect.Response[postv1.CreatePostResponse], error) {
	userIDStr := strings.TrimSpace(req.Msg.UserId)
	content := strings.TrimSpace(req.Msg.Content)

	if userIDStr == "" || content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("userId and content are required"))
	}

	// parse userId into a UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid userId: must be a UUID"))
	}

	insertedPost, err := h.Queries.InsertPost(ctx, repository.InsertPostParams{
		UserID:  userID,
		Content: content,
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("post already exists"))
			case "23503":
				// foreign_key_violation (e.g., user_id references a non-existent user)
				return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
			}
		}
		// Return a sanitized error to avoid leaking DB internals
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create post"))
	}

	// Convert the inserted post to protobuf response
	response := &postv1.CreatePostResponse{
		Post: &postv1.Post{
			Id: insertedPost.ID.String(),
			User: &userv1.User{
				Id: insertedPost.UserID.String(),
			},
			Content:   insertedPost.Content,
			CreatedAt: timestamppb.New(insertedPost.CreatedAt),
			UpdatedAt: timestamppb.New(insertedPost.UpdatedAt),
		},
	}
	return connect.NewResponse(response), nil

}
