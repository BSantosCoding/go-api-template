package dto

import (
	"time"

	"github.com/google/uuid"
)

// GetUserByIdRequest defines the structure for getting a user by id.
type GetUserByIdRequest struct {
	ID        uuid.UUID    `json:"id" validate:"required,uuid"` 
}

// GetUserByEmailRequest defines the structure for getting a user by email.
type GetUserByEmailRequest struct {
	Email        string   `json:"email" validate:"required,email"` 
}

// CreateUserRequest defines the structure for creating a new user.
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name string `json:"name" validate:"omitempty,max=100"`     // Optional field
	Password string `json:"password" validate:"required,min=8"` // Required field
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

// LoginRequest defines the structure for the login request body.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UserResponse defines the standard user data returned to the client.
type UserResponse struct {
	ID        uuid.UUID `json:"id"` // Use uuid.UUID to match your model
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginResponse defines the data returned after successful login.
type LoginResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token,omitempty"` // For future JWT use
}
