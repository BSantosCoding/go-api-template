package dto

import (
	"github.com/google/uuid"
)

// GetUserByIdRequest defines the structure for getting a user by id.
type GetUserByIdRequest struct {
	ID        uuid.UUID    `json:"id" validate:"required,uuid"` 
}

// CreateUserRequest defines the structure for creating a new user.
type CreateUserRequest struct {
	ID        uuid.UUID    `json:"id" validate:"omitempty,uuid"` 
	Email    string `json:"email" validate:"required,email"`
	Name string `json:"name" validate:"omitempty,max=100"`     // Optional field
}

// UpdateUserRequest defines the structure for updating an existing user.
type UpdateUserRequest struct {
	// Email    *string `json:"email" validate:"omitempty,email"` 
	Name *string `json:"name" validate:"omitempty,max=100"`
	ID        uuid.UUID    `json:"id" validate:"required,uuid"`
}

// DeleteUserRequest defines the structure for deleting a user.
type DeleteUserRequest struct {
	ID        uuid.UUID    `json:"id" validate:"required,uuid"` 
}
