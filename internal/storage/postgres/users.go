package postgres

import (
	"context"
	"fmt"
	"log"

	"go-api-template/ent"
	"go-api-template/ent/user"

	// Remove internal models import
	// "go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"golang.org/x/crypto/bcrypt"
)

// UserRepo implements the storage.UserRepository interface using Ent.
type UserRepo struct {
	client *ent.Client
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(client *ent.Client) *UserRepo {
	return &UserRepo{client: client}
}

func (r *UserRepo) WithTx(tx *ent.Tx) storage.UserRepository {
	return &UserRepo{client: tx.Client()}
}

var _ storage.UserRepository = (*UserRepo)(nil)

// GetAll retrieves all users using Ent, returning a slice of *ent.User.
func (r *UserRepo) GetAll(ctx context.Context) ([]*ent.User, error) {
	entUsers, err := r.client.User.
		Query().
		Order(ent.Asc(user.FieldName)). // Order by name (adjust field name if different in schema)
		All(ctx)
	if err != nil {
		log.Printf("Error querying all users with Ent: %v\n", err)
		return nil, err
	}

	// Return Ent users directly
	return entUsers, nil
}

// GetByID retrieves a single user by ID using Ent, returning *ent.User.
func (r *UserRepo) GetByID(ctx context.Context, id *dto.GetUserByIdRequest) (*ent.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(user.IDEQ(id.ID)).
		Only(ctx) // Use Only() to expect exactly one result

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("User not found with ID: %s\n", id.ID)
			return nil, storage.ErrNotFound // Map Ent NotFound error
		}
		log.Printf("Error getting user by ID %s with Ent: %v\n", id.ID, err)
		return nil, err
	}

	return entUser, nil
}

// GetByEmail retrieves a single user by Email using Ent, returning *ent.User (including password hash).
func (r *UserRepo) GetByEmail(ctx context.Context, emailReq *dto.GetUserByEmailRequest) (*ent.User, error) {
	entUser, err := r.client.User.
		Query().
		Where(user.EmailEQ(emailReq.Email)).
		Only(ctx) // Use Only() to expect exactly one result
	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("User not found with email: %s\n", emailReq.Email)
			return nil, storage.ErrNotFound // Map Ent NotFound error
		}
		log.Printf("Error getting user by email %s with Ent: %v\n", emailReq.Email, err)
		return nil, err
	}

	return entUser, nil
}

// Create a new user using Ent, returning the created *ent.User.
func (r *UserRepo) Create(ctx context.Context, userReq *dto.CreateUserRequest) (*ent.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Error hashing password for email %s: %v\n", userReq.Email, err)
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	entUser, err := r.client.User.
		Create().
		SetEmail(userReq.Email).
		SetPasswordHash(string(hashedPassword)).
		SetName(userReq.Name).
		Save(ctx) // Save the new user entity
	if err != nil {
		if ent.IsConstraintError(err) {
			log.Printf("Attempted to create user with duplicate email %s: %v\n", userReq.Email, err)
			return nil, storage.ErrDuplicateEmail // Map to specific duplicate email error
		}
		log.Printf("Error creating user with email %s using Ent: %v\n", userReq.Email, err)
		return nil, err
	}

	log.Printf("User created successfully with ID: %s", entUser.ID)

	return entUser, nil
}

// Update an existing user using Ent, returning the updated *ent.User.
func (r *UserRepo) Update(ctx context.Context, userReq *dto.UpdateUserRequest) (*ent.User, error) {
	updateBuilder := r.client.User.UpdateOneID(userReq.ID)

	// Use nil check for optional fields from DTO pointers
	if userReq.Name != nil {
		updateBuilder.SetName(*userReq.Name)
	}
	// If you allowed email or password updates, add similar checks here

	entUser, err := updateBuilder.Save(ctx) // Save the changes
	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Attempted to update non-existent user %s\n", userReq.ID)
			return nil, storage.ErrNotFound // Map Ent NotFound error
		}
		if ent.IsConstraintError(err) {
			log.Printf("Attempted to update user %s resulting in constraint violation: %v\n", userReq.ID, err)
			return nil, storage.ErrConflict // Map Ent constraint violation error
		}
		log.Printf("Error updating user %s with Ent: %v\n", userReq.ID, err)
		return nil, err
	}

	// Return updated Ent user directly
	return entUser, nil
}

// Delete a user by ID using Ent.
func (r *UserRepo) Delete(ctx context.Context, idReq *dto.DeleteUserRequest) error {
	err := r.client.User.
		DeleteOneID(idReq.ID).
		Exec(ctx) // Execute the delete operation
	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Attempted to delete non-existent user %s\n", idReq.ID)
			return storage.ErrNotFound // Map Ent NotFound error
		}
		log.Printf("Error deleting user %s with Ent: %v\n", idReq.ID, err)
		return err
	}

	return nil
}
