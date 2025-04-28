package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	// Assuming 'id' in DB is UUID type
	ID uuid.UUID `json:"id" db:"id"` // Use uuid.UUID, add db tag if using sqlx/similar

	// Assuming 'name' in DB is VARCHAR/TEXT NOT NULL
	Name string `json:"name" db:"name"`

	// Assuming 'email' in DB is VARCHAR/TEXT UNIQUE NOT NULL
	Email string `json:"email" db:"email"`

	// Assuming 'created_at' in DB is TIMESTAMPTZ NOT NULL
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Assuming 'updated_at' in DB is TIMESTAMPTZ NOT NULL
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Item represents an item in the system
type Item struct {
	// Assuming 'id' in DB is UUID type
	ID uuid.UUID `json:"id" db:"id"`

	// Assuming 'name' in DB is VARCHAR/TEXT NOT NULL
	Name string `json:"name" db:"name"`

	// Assuming 'description' in DB is TEXT NULLABLE
	Description *string `json:"description,omitempty" db:"description"` // Pointer for NULLable string

	Price float64 `json:"price" db:"price"`

	Attributes map[string]interface{} `json:"attributes,omitempty" db:"attributes"` // nil map if NULL
	
	// Assuming 'created_at' in DB is TIMESTAMPTZ NOT NULL
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Assuming 'updated_at' in DB is TIMESTAMPTZ NOT NULL
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

