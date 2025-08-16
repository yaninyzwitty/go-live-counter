package handlers

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	userv1 "github.com/yaninyzwitty/go-live-counter/gen/user/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/user/v1/userv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserStoreServiceHandler implements the UserService API.

type UserStoreServiceHandler struct {
	userv1connect.UnimplementedUserServiceHandler
	Queries *repository.Queries
}

// CREATE new instance of user handler
func New(queries *repository.Queries) *UserStoreServiceHandler {
	return &UserStoreServiceHandler{
		Queries: queries,
	}
}

func (h *UserStoreServiceHandler) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {

	// validation
	if req.Msg.Name == "" || req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and email are required"))
	}

	// insert user to the database
	insertedUser, err := h.Queries.InsertItem(ctx, repository.InsertItemParams{
		Name: req.Msg.Name,
		Email: pgtype.Text{
			String: req.Msg.Email,
			Valid:  true,
		},
	})

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert the inserted user to protobuf response
	response := &userv1.CreateUserResponse{
		User: &userv1.User{
			Id:        insertedUser.ID.String(),
			Name:      insertedUser.Name,
			Email:     insertedUser.Email.String,
			CreatedAt: timestamppb.New(insertedUser.CreatedAt),
			UpdatedAt: timestamppb.New(insertedUser.UpdatedAt),
		},
	}

	return connect.NewResponse(response), nil
}
