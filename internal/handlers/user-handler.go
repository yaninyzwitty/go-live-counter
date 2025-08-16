package handlers

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgconn"
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
	name := strings.TrimSpace(req.Msg.Name)
	email := strings.TrimSpace(req.Msg.Email)

	if name == "" || email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name and email are required"))
	}

	// insert user to the database
	insertedUser, err := h.Queries.InsertItem(ctx, repository.InsertItemParams{
		Name: name,
		Email: pgtype.Text{
			String: email,
			Valid:  true,
		},
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("user with that email already exists"))
		}
		// Return a sanitized error to avoid leaking DB internals
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create user"))
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
