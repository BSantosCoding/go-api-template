package storage

import (
	"context"
	"go-api-template/internal/models"
	"go-api-template/internal/transport/dto"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	GetAll(ctx context.Context) ([]models.User, error)
	GetByID(ctx context.Context, id *dto.GetUserByIdRequest) (*models.User, error)
	GetByEmail(ctx context.Context, id *dto.GetUserByEmailRequest) (*models.User, error)
	Create(ctx context.Context, user *dto.CreateUserRequest) (*models.User, error) // Modify to return created user ID or full user if needed
	Update(ctx context.Context, user *dto.UpdateUserRequest) (*models.User, error) // Modify to return updated user if needed
	Delete(ctx context.Context, id *dto.DeleteUserRequest) error
}

// ItemRepository defines the interface for item data operations.
type ItemRepository interface {
	GetAll(ctx context.Context) ([]models.Item, error)
	GetByID(ctx context.Context, id string) (*models.Item, error)
	Create(ctx context.Context, item *models.Item) error // Modify as needed
	Update(ctx context.Context, id string, item *models.Item) error // Modify as needed
	Delete(ctx context.Context, id string) error
}
