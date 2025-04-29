package postgres

import (
	"context"
	"errors" // Import errors package
	"fmt"
	"log" // For logging errors

	"go-api-template/internal/models"
	"go-api-template/internal/storage" // Import the interface package
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // For checking specific errors
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// UserRepo implements the storage.UserRepository interface using PostgreSQL.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Compile-time check to ensure UserRepo implements UserRepository
var _ storage.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) GetAll(ctx context.Context) ([]models.User, error) {
	query := `SELECT id, name, email FROM users ORDER BY name ASC;`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		log.Printf("Error querying all users: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.User])
	if err != nil {
		log.Printf("Error scanning users: %v\n", err)
		return nil, err
	}

	// Return empty slice instead of nil if no users found
	if users == nil {
		users = []models.User{}
	}

	return users, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id *dto.GetUserByIdRequest) (*models.User, error) {
	query := `SELECT id, name, email FROM users WHERE id = $1;`
	row := r.db.QueryRow(ctx, query, id.ID)

	var user models.User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound // Use a custom error type later if needed
		}
		log.Printf("Error scanning user by ID %s: %v\n", id, err)
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a single user by Email, including the password hash.
func (r *UserRepo) GetByEmail(ctx context.Context, email *dto.GetUserByEmailRequest) (*models.User, error) {
	// Select all fields needed for authentication comparison
	query := `SELECT id, name, email, password_hash, created_at, updated_at FROM users WHERE email = $1;`
	row := r.db.QueryRow(ctx, query, email.Email)

	var user models.User
	// Scan all fields, including password hash
	err := row.Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PasswordHash, // Include password hash
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("User not found with email: %s\n", email.Email)
			return nil, storage.ErrNotFound // Use custom error
		}
		log.Printf("Error scanning user by email %s: %v\n", email.Email, err)
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) Create(ctx context.Context, userReq *dto.CreateUserRequest) (*models.User, error) {
	// --- Password Hashing ---
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password for email %s: %v\n", userReq.Email, err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	// ------------------------

	// Include password_hash in the insert statement
	// Use NOW() for timestamps assuming DB columns are TIMESTAMPTZ
	sql := `INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
             VALUES ($1, $2, $3, $4, NOW(), NOW())
             RETURNING id, name, email, created_at, updated_at` // Return safe fields

	createdUser := &models.User{} // To store the returned values

	// Execute the query, passing the hashed password
	err = r.db.QueryRow(ctx, sql, uuid.New(), userReq.Name, userReq.Email, string(hashedPassword)).Scan(
		&createdUser.ID,
		&createdUser.Name,
		&createdUser.Email,
		&createdUser.CreatedAt,
		&createdUser.UpdatedAt,
		// Note: We are NOT returning/scanning the password_hash back
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			// Check constraint name to be more specific (optional but recommended)
			// Common constraint names: users_email_key, users_email_unique, users_pkey
			if pgErr.ConstraintName == "users_email_key" || pgErr.ConstraintName == "users_email_unique" {
				log.Printf("Attempted to create user with duplicate email %s: %v\n", userReq.Email, err)
				return nil, storage.ErrDuplicateEmail // Specific error for email
			}
			// Could be duplicate ID or other unique constraint
			log.Printf("Attempted to create user with duplicate unique field (ID or other): %v\n", err)
			return nil, storage.ErrConflict // General conflict error
		}
		// Log and return other errors
		log.Printf("Error creating user with email %s: %v\n", userReq.Email, err)
		return nil, err
	}

	log.Printf("User created successfully with ID: %s", createdUser.ID)
	return createdUser, nil
}

func (r *UserRepo) Update(ctx context.Context, user *dto.UpdateUserRequest) (*models.User, error) {
	sql := `UPDATE users
             SET name = $1
             WHERE id = $2
             RETURNING id, name, email, created_at, updated_at` // Return all needed fields

	updatedUser := &models.User{}

	err := r.db.QueryRow(ctx, sql, user.Name, user.ID).Scan( // Pass values for SET and WHERE
        &updatedUser.ID,
        &updatedUser.Name,
        &updatedUser.Email,
        &updatedUser.CreatedAt,
        &updatedUser.UpdatedAt, // This will contain the trigger-set value
    )
	if err != nil {
		// Check for unique constraint violation on update (e.g., email)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			log.Printf("Attempted to update user %s resulting in duplicate email: %v\n", user.ID, err)
			return nil, storage.ErrConflict
		}
		log.Printf("Error updating user %s: %v\n", user.ID, err)
		return nil, err
	}

	return updatedUser, nil
}

func (r *UserRepo) Delete(ctx context.Context, id *dto.DeleteUserRequest) error {
	query := `DELETE FROM users WHERE id = $1;`

	cmdTag, err := r.db.Exec(ctx, query, id.ID)
	if err != nil {
		log.Printf("Error deleting user %s: %v\n", id, err)
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return storage.ErrNotFound // No user found with that ID
	}

	return nil
}
