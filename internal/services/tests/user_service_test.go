package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mock_storage "go-api-template/internal/mocks"
	"go-api-template/internal/models"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtSecret   = "test-secret-key"
	jwtDuration = 15 * time.Minute
)

// Helper to create a pointer to a string
func ptr(s string) *string { return &s }

func TestUserService_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	ctx := context.Background()

	req := &dto.CreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)

	expectedUser := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: "hashedpassword", // Repo handles hashing
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	mockUserRepo.EXPECT().
		Create(ctx, req). // Expecting the DTO
		Return(expectedUser, nil).
		Times(1)

	user, err := userService.Register(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedUser, user)
}

func TestUserService_Register_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	ctx := context.Background()
	req := &dto.CreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)

	// Mock repo returning a conflict error
	mockUserRepo.EXPECT().
		Create(ctx, req).
		Return(nil, storage.ErrDuplicateEmail). // Repo returns storage error
		Times(1)

	_, err := userService.Register(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrConflict))
}

func TestUserService_Register_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	ctx := context.Background()
	req := &dto.CreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	repoErr := errors.New("database connection lost")

	mockUserRepo.EXPECT().Create(ctx, req).Return(nil, repoErr).Times(1)

	_, err := userService.Register(ctx, req)

	require.Error(t, err)
	assert.False(t, errors.Is(err, services.ErrConflict)) // Ensure it's not mapped to conflict
	assert.Contains(t, err.Error(), "internal error creating user") // Check if wrapped
	assert.True(t, errors.Is(err, repoErr)) // Check if original error is wrapped
}

func TestUserService_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	loginReq := &dto.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	// Hash the password for the mock user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(loginReq.Password), bcrypt.DefaultCost)

	mockUser := &models.User{
		ID:           uuid.New(),
		Email:        loginReq.Email,
		PasswordHash: string(hashedPassword),
		Name:         "Test User",
	}

	// Mock GetByEmail call
	mockUserRepo.EXPECT().
		GetByEmail(ctx, gomock.Any()). // Could be more specific with matcher if needed
		DoAndReturn(func(ctx context.Context, getReq *dto.GetUserByEmailRequest) (*models.User, error) {
			assert.Equal(t, loginReq.Email, getReq.Email)
			return mockUser, nil
		}).Times(1)

	user, token, err := userService.Login(ctx, loginReq)

	require.NoError(t, err)
	assert.Equal(t, mockUser.ID, user.ID)
	assert.NotEmpty(t, token)
	// You could potentially decode the token here to verify claims if needed
}

func TestUserService_Login_InvalidPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	loginReq := &dto.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	mockUser := &models.User{
		ID:           uuid.New(),
		Email:        loginReq.Email,
		PasswordHash: string(hashedPassword),
	}

	mockUserRepo.EXPECT().GetByEmail(ctx, gomock.Any()).Return(mockUser, nil)

	_, _, err := userService.Login(ctx, loginReq)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	loginReq := &dto.LoginRequest{
		Email:    "notfound@example.com",
		Password: "password123",
	}

	// Mock repo returning not found
	mockUserRepo.EXPECT().GetByEmail(ctx, gomock.Any()).Return(nil, storage.ErrNotFound) // Repo returns storage error

	_, _, err := userService.Login(ctx, loginReq)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials)) // Service maps NotFound to InvalidCredentials
}

func TestUserService_Login_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	loginReq := &dto.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}
	repoErr := errors.New("db connection error")

	// Mock repo returning a generic error
	mockUserRepo.EXPECT().GetByEmail(ctx, gomock.Any()).Return(nil, repoErr)

	_, _, err := userService.Login(ctx, loginReq)

	require.Error(t, err)
	assert.False(t, errors.Is(err, services.ErrInvalidCredentials)) // Ensure not mapped
	assert.Contains(t, err.Error(), "internal error during login")
}

func TestUserService_Update_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	ctx := context.Background()

	userID := uuid.New()
	req := &dto.UpdateUserRequest{
		ID:   userID, // User is updating themselves
		Name: ptr("Updated Name"),
	}

	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)

	updatedUser := &models.User{ID: userID, Name: "Updated Name", Email: "test@example.com"}

	// Mock Update call
	mockUserRepo.EXPECT().Update(ctx, req).Return(updatedUser, nil).Times(1)

	user, err := userService.Update(ctx, req) // Pass requestingUserID == req.ID
	// Note: Based on provided code, Update doesn't take requestingUserID.
	// If the code *was* updated previously, this test call should be:
	// user, err := userService.Update(ctx, userID, req)

	require.NoError(t, err)
	assert.Equal(t, updatedUser, user)
}

// Add TestUserService_Update_Forbidden if/when authorization is added back

func TestUserService_Update_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	req := &dto.UpdateUserRequest{
		ID:   uuid.New(),
		Name: ptr("Updated Name"),
	}

	mockUserRepo.EXPECT().Update(ctx, req).Return(nil, storage.ErrNotFound).Times(1)

	_, err := userService.Update(ctx, req)

	require.Error(t, err)
	// Assuming Update just passes through repo errors for now
	assert.True(t, errors.Is(err, storage.ErrNotFound))
	// If service mapped it: assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestUserService_Update_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()
	repoErr := errors.New("db write failed")

	req := &dto.UpdateUserRequest{
		ID:   uuid.New(),
		Name: ptr("Updated Name"),
	}

	mockUserRepo.EXPECT().Update(ctx, req).Return(nil, repoErr).Times(1)

	_, err := userService.Update(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Assuming pass-through
}

func TestUserService_Delete_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	userID := uuid.New() // User deleting themselves

	// Mock Delete call
	mockUserRepo.EXPECT().
		Delete(ctx, gomock.Any()). // Check the DTO passed
		DoAndReturn(func(ctx context.Context, req *dto.DeleteUserRequest) error {
			assert.Equal(t, userID, req.ID)
			return nil // Success
		}).Times(1)
	req := &dto.DeleteUserRequest{ID: userID} // User is deleting themselves
	err := userService.Delete(ctx, req) // Pass same ID for requesting and target

	require.NoError(t, err)
}

func TestUserService_Delete_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	req := &dto.DeleteUserRequest{ID: uuid.New()}

	mockUserRepo.EXPECT().Delete(ctx, req).Return(storage.ErrNotFound).Times(1)

	err := userService.Delete(ctx, req)

	require.Error(t, err)
	// Assuming Update just passes through repo errors for now
	assert.True(t, errors.Is(err, storage.ErrNotFound))
	// If service mapped it: assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestUserService_Delete_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()
	repoErr := errors.New("db constraint violation")
	req := &dto.DeleteUserRequest{ID: uuid.New()}

	mockUserRepo.EXPECT().Delete(ctx, req).Return(repoErr).Times(1)

	err := userService.Delete(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Assuming pass-through
}

func TestUserService_GetAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	expectedUsers := []models.User{
		{ID: uuid.New(), Email: "user1@example.com", Name: "User One"},
		{ID: uuid.New(), Email: "user2@example.com", Name: "User Two"},
	}

	mockUserRepo.EXPECT().GetAll(ctx).Return(expectedUsers, nil).Times(1)

	users, err := userService.GetAll(ctx)

	require.NoError(t, err)
	assert.Equal(t, expectedUsers, users)
}

func TestUserService_GetAll_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()
	repoErr := errors.New("db read error")

	mockUserRepo.EXPECT().GetAll(ctx).Return(nil, repoErr).Times(1)

	_, err := userService.GetAll(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Assuming pass-through
}

func TestUserService_GetByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	userID := uuid.New()
	req := &dto.GetUserByIdRequest{ID: userID}
	expectedUser := &models.User{ID: userID, Email: "test@example.com", Name: "Test User"}

	mockUserRepo.EXPECT().
		GetByID(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *dto.GetUserByIdRequest) (*models.User, error) {
			assert.Equal(t, userID, req.ID)
			return expectedUser, nil
		}).Times(1)

	user, err := userService.GetByID(ctx, req)
	// Note: Based on provided code, GetByID takes DTO.
	// If the code *was* updated previously to take ID, this test call should be:
	// user, err := userService.GetByID(ctx, userID)
	// And the mock expectation would change slightly.

	require.NoError(t, err)
	assert.Equal(t, expectedUser, user)
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	req := &dto.GetUserByIdRequest{ID: uuid.New()}

	mockUserRepo.EXPECT().GetByID(ctx, req).Return(nil, storage.ErrNotFound).Times(1) // Repo returns storage error

	_, err := userService.GetByID(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestUserService_GetByID_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()
	repoErr := errors.New("db connection failed")
	req := &dto.GetUserByIdRequest{ID: uuid.New()}

	mockUserRepo.EXPECT().GetByID(ctx, req).Return(nil, repoErr).Times(1)

	_, err := userService.GetByID(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Assuming pass-through, not mapped
}

func TestUserService_GetByEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	email := "findme@example.com"
	req := &dto.GetUserByEmailRequest{Email: email}
	expectedUser := &models.User{ID: uuid.New(), Email: email, Name: "Find Me"}

	mockUserRepo.EXPECT().
		GetByEmail(ctx, gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *dto.GetUserByEmailRequest) (*models.User, error) {
			assert.Equal(t, email, req.Email)
			return expectedUser, nil
		}).Times(1)

	user, err := userService.GetByEmail(ctx, req)
	// Note: Based on provided code, GetByEmail takes DTO.
	// If the code *was* updated previously to take string, this test call should be:
	// user, err := userService.GetByEmail(ctx, email)
	// And the mock expectation would change slightly.

	require.NoError(t, err)
	assert.Equal(t, expectedUser, user)
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()

	req := &dto.GetUserByEmailRequest{Email: "notfound@example.com"}

	mockUserRepo.EXPECT().GetByEmail(ctx, req).Return(nil, storage.ErrNotFound).Times(1) // Repo returns storage error

	_, err := userService.GetByEmail(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound)) // Service maps this one
}

func TestUserService_GetByEmail_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	userService := services.NewUserService(mockUserRepo, jwtSecret, jwtDuration)
	ctx := context.Background()
	repoErr := errors.New("db connection failed")
	req := &dto.GetUserByEmailRequest{Email: "test@example.com"}

	mockUserRepo.EXPECT().GetByEmail(ctx, req).Return(nil, repoErr).Times(1)

	_, err := userService.GetByEmail(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Assuming pass-through, not mapped
}