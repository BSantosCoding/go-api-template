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

	"github.com/go-redis/redismock/v9" // Re-import redismock
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9" // Import redis for redis.Nil
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtSecret              = "test-secret-key"
	jwtDuration            = 15 * time.Minute
	refreshTokenDuration   = 72 * time.Hour
)

var (
	testUserID = uuid.New() // Use a consistent ID for predictable mocks/results
)

// Helper to create a pointer to a string
func ptr(s string) *string { return &s }

func TestUserService_Register(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client as it's not used in Register
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)

	repoErrDbConnectionLost := errors.New("database connection lost")

	tests := []struct {
		name          string
		req           *dto.CreateUserRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.CreateUserRequest)
		expectedUser  *models.User
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.CreateUserRequest) {
				mockReturnUser := &models.User{
					ID:           testUserID,
					Email:        req.Email,
					Name:         req.Name,
					PasswordHash: "hashedpassword", // Repo handles hashing
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				repo.EXPECT().Create(gomock.Any(), req).Return(mockReturnUser, nil).Times(1)
			},
			expectedUser: &models.User{
				ID:    testUserID,
				Email: "test@example.com",
				Name:  "Test User",
				// PasswordHash is not returned by the service on Register
			},
			expectedError: nil,
		},
		{
			name: "Conflict - Duplicate Email",
			req: &dto.CreateUserRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.CreateUserRequest) {
				repo.EXPECT().Create(gomock.Any(), req).Return(nil, storage.ErrDuplicateEmail).Times(1)
			},
			expectedUser:  nil,
			expectedError: services.ErrConflict,
		},
		{
			name: "Repository Error",
			req: &dto.CreateUserRequest{
				Email:    "error@example.com",
				Password: "password123",
				Name:     "Error User",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.CreateUserRequest) {
				repo.EXPECT().Create(gomock.Any(), req).Return(nil, repoErrDbConnectionLost).Times(1)
			},
			expectedUser:  nil,
			expectedError: repoErrDbConnectionLost,
			errorContains: "internal error creating user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tt.mockSetup(mockUserRepo, tt.req)

			user, err := userService.Register(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.expectedUser.ID, user.ID)
				assert.Equal(t, tt.expectedUser.Email, user.Email)
				assert.Equal(t, tt.expectedUser.Name, user.Name)
				// Don't assert PasswordHash as it's not returned by Register
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	ctx := context.Background()

	correctPassword := "password123"
	correctHashedPassword, _ := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)
	repoErrDbConnection := errors.New("db connection error")

	tests := []struct {
		name          string
		req           *dto.LoginRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.LoginRequest)
		expectedUser  *models.User
		expectRefreshToken bool
		expectToken   bool
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.LoginRequest{
				Email:    "test@example.com",
				Password: correctPassword,
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.LoginRequest) {
				mockReturnUser := &models.User{
					ID:           testUserID,
					Email:        req.Email,
					PasswordHash: string(correctHashedPassword),
					Name:         "Test User",
				}
				repo.EXPECT().GetByEmail(gomock.Any(), &dto.GetUserByEmailRequest{Email: req.Email}).Return(mockReturnUser, nil).Times(1)
				// Expect Redis Set for refresh token
				// Expectation will be set inside t.Run using redismock.Regexp()


			},
			expectedUser: &models.User{
				ID: testUserID,
				Email: "test@example.com",
				Name:  "Test User",
			},
			expectToken:   true,
			expectRefreshToken: true,
			expectedError: nil,
		},
		{
			name: "Invalid Password",
			req: &dto.LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.LoginRequest) {
				mockReturnUser := &models.User{
					ID:           testUserID,
					Email:        req.Email,
					PasswordHash: string(correctHashedPassword),
					Name:         "Test User",
				}
				repo.EXPECT().GetByEmail(gomock.Any(), &dto.GetUserByEmailRequest{Email: req.Email}).Return(mockReturnUser, nil).Times(1)
			},
			expectedUser:  nil,
			expectToken:   false,
			expectRefreshToken: false,
			expectedError: services.ErrInvalidCredentials,
		},
		{
			name: "User Not Found",
			req: &dto.LoginRequest{
				Email:    "notfound@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.LoginRequest) {
				repo.EXPECT().GetByEmail(gomock.Any(), &dto.GetUserByEmailRequest{Email: req.Email}).Return(nil, storage.ErrNotFound).Times(1)
			},
			expectedUser:  nil,
			expectToken:   false,
			expectRefreshToken: false,
			expectedError: services.ErrInvalidCredentials,
		},
		{
			name: "Repository Error on GetByEmail",
			req: &dto.LoginRequest{
				Email:    "error@example.com",
				Password: "password123",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.LoginRequest) {
				repo.EXPECT().GetByEmail(gomock.Any(), &dto.GetUserByEmailRequest{Email: req.Email}).Return(nil, repoErrDbConnection).Times(1)
			},
			expectedUser:  nil,
			expectToken:   false,
			expectRefreshToken: false,
			expectedError: repoErrDbConnection,
			errorContains: "internal error during login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Provide a mock Redis client to prevent nil pointer dereference in Login
			mockRedisClient, mockRedis := redismock.NewClientMock()
			userService := services.NewUserService(mockUserRepo, mockRedisClient, jwtSecret, jwtDuration, refreshTokenDuration)

			tt.mockSetup(mockUserRepo, tt.req)
			if tt.name == "Success" {
				mockRedis.Regexp().ExpectSet(services.RedisRefreshTokenPrefix+".*", testUserID.String(), refreshTokenDuration).SetVal("OK") // Use redismock.Regexp() as argument
			}
			user, accessToken, refreshToken, err := userService.Login(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, user)
				assert.Empty(t, accessToken)
				assert.Empty(t, refreshToken)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.expectedUser.ID, user.ID)
				assert.Equal(t, tt.expectedUser.Email, user.Email)
				assert.Equal(t, tt.expectedUser.Name, user.Name)
				if tt.expectToken {
					assert.NotEmpty(t, accessToken)
				} else {
					assert.Empty(t, accessToken)
				}
				if tt.expectRefreshToken {
					assert.NotEmpty(t, refreshToken)
				} else {
					assert.Empty(t, refreshToken)
				}
			}
			// Verify all Redis expectations were met
			assert.NoError(t, mockRedis.ExpectationsWereMet())
		})
	}
}

func TestUserService_Refresh(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl) // Not used directly but needed for NewUserService

	validRefreshToken := "valid-refresh-token"
	redisErrGeneric := errors.New("redis connection error")

	tests := []struct {
		name                 string
		refreshToken         string
		mockSetup            func(mockRedis redismock.ClientMock, token string)
		expectNewAccessToken bool
		expectNewRefreshToken bool
		expectedError        error
		errorContains        string
	}{
		{
			name:         "Success",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectGet(key).SetVal(testUserID.String())
				mockRedis.ExpectDel(key).SetVal(1)
				// Expect Set for the new token
				mockRedis.Regexp().ExpectSet(services.RedisRefreshTokenPrefix+".*", testUserID.String(), refreshTokenDuration).SetVal("OK")
			},
			expectNewAccessToken: true,
			expectNewRefreshToken: true,
			expectedError:        nil,
		},
		{
			name:         "Refresh Token Not Found",
			refreshToken: "invalid-token",
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectGet(key).SetErr(redis.Nil) // Simulate token not found
			},
			expectNewAccessToken: false,
			expectNewRefreshToken: false,
			expectedError:        services.ErrInvalidCredentials,
		},
		{
			name:         "Redis Error on Get",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectGet(key).SetErr(redisErrGeneric)
			},
			expectNewAccessToken: false,
			expectNewRefreshToken: false,
			expectedError:        redisErrGeneric,
			errorContains:        "internal error validating refresh token",
		},
		{
			name:         "Redis Error on Del (should still proceed)",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectGet(key).SetVal(testUserID.String())
				mockRedis.ExpectDel(key).SetErr(redisErrGeneric)
				// Still expect Set for the new token
				mockRedis.Regexp().ExpectSet(services.RedisRefreshTokenPrefix+".*", testUserID.String(), refreshTokenDuration).SetVal("OK")
			},
			expectNewAccessToken: true, // Should still issue new tokens
			expectNewRefreshToken: true,
			expectedError:        nil, // No error returned to caller in this case
		},
		{
			name:         "Redis Error on Set New Token",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectGet(key).SetVal(testUserID.String())
				mockRedis.ExpectDel(key).SetVal(1)
				// Expect Set for the new token to fail
				mockRedis.Regexp().ExpectSet(services.RedisRefreshTokenPrefix+".*", testUserID.String(), refreshTokenDuration).SetErr(redisErrGeneric)
			},
			expectNewAccessToken: false,
			expectNewRefreshToken: false, // Fails because storing new refresh token failed
			expectedError:        redisErrGeneric,
			errorContains:        "failed to handle new refresh token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedisClient, mockRedis := redismock.NewClientMock()
			userService := services.NewUserService(mockUserRepo, mockRedisClient, jwtSecret, jwtDuration, refreshTokenDuration)

			tt.mockSetup(mockRedis, tt.refreshToken)
			ctx := context.Background()

			refreshReq := &dto.RefreshRequest{RefreshToken: tt.refreshToken}
			newAccessToken, newRefreshToken, err := userService.Refresh(ctx, refreshReq)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError) || err.Error() == tt.expectedError.Error(), "Expected error %v or similar, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Empty(t, newAccessToken)
				assert.Empty(t, newRefreshToken)
			} else {
				require.NoError(t, err)
				if tt.expectNewAccessToken {
					assert.NotEmpty(t, newAccessToken)
				} else {
					assert.Empty(t, newAccessToken)
				}
				if tt.expectNewRefreshToken {
					assert.NotEmpty(t, newRefreshToken)
				} else {
					assert.Empty(t, newRefreshToken)
				}
			}
			// Verify all Redis expectations were met
			assert.NoError(t, mockRedis.ExpectationsWereMet())
		})
	}
}

func TestUserService_Update(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)
	ctx := context.Background()

	repoErrDbWriteFailed := errors.New("db write failed")

	tests := []struct {
		name          string
		req           *dto.UpdateUserRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.UpdateUserRequest)
		expectedUser  *models.User
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.UpdateUserRequest{
				ID:   testUserID,
				Name: ptr("Updated Name"),
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.UpdateUserRequest) {
				mockReturnUser := &models.User{
					ID:        req.ID,
					Name:      *req.Name,
					Email:     "original@example.com", // Email shouldn't change in this specific test case
					UpdatedAt: time.Now(),
				}
				repo.EXPECT().Update(gomock.Any(), req).Return(mockReturnUser, nil).Times(1)
			},
			expectedUser: &models.User{
				ID:    testUserID,
				Name:  "Updated Name",
				Email: "original@example.com",
			},
			expectedError: nil,
		},
		{
			name: "Not Found",
			req: &dto.UpdateUserRequest{
				ID:   uuid.New(),
				Name: ptr("Updated Name"),
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.UpdateUserRequest) {
				repo.EXPECT().Update(gomock.Any(), req).Return(nil, services.ErrNotFound).Times(1)
			},
			expectedUser:  nil,
			expectedError: services.ErrNotFound,
		},
		{
			name: "Repository Error",
			req: &dto.UpdateUserRequest{
				ID:   testUserID,
				Name: ptr("Error Update"),
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.UpdateUserRequest) {
				repo.EXPECT().Update(gomock.Any(), req).Return(nil, repoErrDbWriteFailed).Times(1)
			},
			expectedUser:  nil,
			expectedError: repoErrDbWriteFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockUserRepo, tt.req)

			user, err := userService.Update(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.expectedUser.ID, user.ID)
				assert.Equal(t, tt.expectedUser.Name, user.Name)
				assert.Equal(t, tt.expectedUser.Email, user.Email)
			}
		})
	}
}

func TestUserService_Logout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)

	validRefreshToken := "valid-refresh-token-to-logout"
	redisErrGeneric := errors.New("redis connection error during logout")

	tests := []struct {
		name          string
		refreshToken  string
		mockSetup     func(mockRedis redismock.ClientMock, token string)
		expectedError error
		errorContains string
	}{
		{
			name:         "Success",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectDel(key).SetVal(1)
			},
			expectedError: nil,
		},
		{
			name:         "Token Not Found (Still Success)",
			refreshToken: "already-logged-out-token",
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectDel(key).SetErr(redis.Nil)
			},
			expectedError: nil, // Logout succeeds even if token is already gone
		},
		{
			name:         "Redis Error on Del",
			refreshToken: validRefreshToken,
			mockSetup: func(mockRedis redismock.ClientMock, token string) {
				key := services.RedisRefreshTokenPrefix + token
				mockRedis.ExpectDel(key).SetErr(redisErrGeneric) // Simulate Redis error
			}, // Simulate Redis error
			expectedError: redisErrGeneric,
			errorContains: "failed to invalidate session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedisClient, mockRedis := redismock.NewClientMock()
			userService := services.NewUserService(mockUserRepo, mockRedisClient, jwtSecret, jwtDuration, refreshTokenDuration)
			tt.mockSetup(mockRedis, tt.refreshToken)
			ctx := context.Background()

			logoutReq := &dto.LogoutRequest{RefreshToken: tt.refreshToken}
			err := userService.Logout(ctx, logoutReq)

			assert.True(t, errors.Is(err, tt.expectedError) || (err != nil && tt.expectedError != nil && err.Error() == tt.expectedError.Error()), "Expected error %v or similar, got %v", tt.expectedError, err)
			assert.NoError(t, mockRedis.ExpectationsWereMet())
		})
	}
}

func TestUserService_Delete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)
	ctx := context.Background()

	repoErrDbConstraintViolation := errors.New("db constraint violation")

	tests := []struct {
		name          string
		req           *dto.DeleteUserRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.DeleteUserRequest)
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.DeleteUserRequest{
				ID: testUserID,
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.DeleteUserRequest) {
				repo.EXPECT().Delete(gomock.Any(), req).Return(nil).Times(1)
			},
			expectedError: nil,
		},
		{
			name: "Not Found",
			req: &dto.DeleteUserRequest{
				ID: uuid.New(),
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.DeleteUserRequest) {
				repo.EXPECT().Delete(gomock.Any(), req).Return(services.ErrNotFound).Times(1)
			},
			expectedError: services.ErrNotFound,
		},
		{
			name: "Repository Error",
			req: &dto.DeleteUserRequest{
				ID: testUserID,
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.DeleteUserRequest) {
				repo.EXPECT().Delete(gomock.Any(), req).Return(repoErrDbConstraintViolation).Times(1)
			},
			expectedError: repoErrDbConstraintViolation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockUserRepo, tt.req)

			err := userService.Delete(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUserService_GetAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)
	ctx := context.Background()

	repoErrDbReadError := errors.New("db read error")
	getAllUserID1 := uuid.New()

	tests := []struct {
		name           string
		mockSetup      func(repo *mock_storage.MockUserRepository)
		expectedUsers  []models.User
		expectedError  error
		errorContains  string
	}{
		{
			name: "Success",
			mockSetup: func(repo *mock_storage.MockUserRepository) {
				mockReturnUsers := []models.User{
					{ID: getAllUserID1, Email: "user1@example.com", Name: "User One"},
					{ID: testUserID, Email: "user2@example.com", Name: "User Two"},
				}
				repo.EXPECT().GetAll(gomock.Any()).Return(mockReturnUsers, nil).Times(1)
			},
			expectedUsers: []models.User{
				{ID: getAllUserID1, Email: "user1@example.com", Name: "User One"},
				{ID: testUserID, Email: "user2@example.com", Name: "User Two"},
			},
			expectedError: nil,
		},
		{
			name: "Repository Error",
			mockSetup: func(repo *mock_storage.MockUserRepository) {
				repo.EXPECT().GetAll(gomock.Any()).Return(nil, repoErrDbReadError).Times(1)
			},
			expectedUsers: nil,
			expectedError: repoErrDbReadError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockUserRepo)

			users, err := userService.GetAll(ctx)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, users)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUsers, users)
			}
		})
	}
}

func TestUserService_GetByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)
	ctx := context.Background()

	repoErrDbConnectionFailed := errors.New("db connection failed")

	tests := []struct {
		name          string
		req           *dto.GetUserByIdRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.GetUserByIdRequest)
		expectedUser  *models.User
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.GetUserByIdRequest{
				ID: testUserID,
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByIdRequest) {
				mockReturnUser := &models.User{
					ID:    req.ID,
					Email: "test@example.com",
					Name:  "Test User",
				}
				repo.EXPECT().GetByID(gomock.Any(), req).Return(mockReturnUser, nil).Times(1)
			},
			expectedUser: &models.User{
				ID:    testUserID,
				Email: "test@example.com",
				Name:  "Test User",
			},
			expectedError: nil,
		},
		{
			name: "Not Found",
			req: &dto.GetUserByIdRequest{
				ID: uuid.New(),
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByIdRequest) {
				repo.EXPECT().GetByID(gomock.Any(), req).Return(nil, storage.ErrNotFound).Times(1)
			},
			expectedUser:  nil,
			expectedError: services.ErrNotFound, // Service maps this
		},
		{
			name: "Repository Error",
			req: &dto.GetUserByIdRequest{
				ID: testUserID,
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByIdRequest) {
				repo.EXPECT().GetByID(gomock.Any(), req).Return(nil, repoErrDbConnectionFailed).Times(1)
			},
			expectedUser:  nil,
			expectedError: repoErrDbConnectionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockUserRepo, tt.req)

			user, err := userService.GetByID(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUser, user)
			}
		})
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUserRepo := mock_storage.NewMockUserRepository(ctrl)
	// Pass nil for Redis client
	userService := services.NewUserService(mockUserRepo, nil, jwtSecret, jwtDuration, refreshTokenDuration)
	ctx := context.Background()

	repoErrDbConnectionFailed := errors.New("db connection failed")

	tests := []struct {
		name          string
		req           *dto.GetUserByEmailRequest
		mockSetup     func(repo *mock_storage.MockUserRepository, req *dto.GetUserByEmailRequest)
		expectedUser  *models.User
		expectedError error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.GetUserByEmailRequest{
				Email: "findme@example.com",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByEmailRequest) {
				mockReturnUser := &models.User{
					ID:    testUserID,
					Email: req.Email,
					Name:  "Find Me",
				}
				repo.EXPECT().GetByEmail(gomock.Any(), req).Return(mockReturnUser, nil).Times(1)
			},
			expectedUser: &models.User{
				ID:    testUserID,
				Email: "findme@example.com",
				Name:  "Find Me",
			},
			expectedError: nil,
		},
		{
			name: "Not Found",
			req: &dto.GetUserByEmailRequest{
				Email: "notfound@example.com",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByEmailRequest) {
				repo.EXPECT().GetByEmail(gomock.Any(), req).Return(nil, storage.ErrNotFound).Times(1)
			},
			expectedUser:  nil,
			expectedError: services.ErrNotFound, // Service maps this
		},
		{
			name: "Repository Error",
			req: &dto.GetUserByEmailRequest{
				Email: "error@example.com",
			},
			mockSetup: func(repo *mock_storage.MockUserRepository, req *dto.GetUserByEmailRequest) {
				repo.EXPECT().GetByEmail(gomock.Any(), req).Return(nil, repoErrDbConnectionFailed).Times(1)
			},
			expectedUser:  nil,
			expectedError: repoErrDbConnectionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup(mockUserRepo, tt.req)

			user, err := userService.GetByEmail(ctx, tt.req)

			if tt.expectedError != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedError), "Expected error %v, got %v", tt.expectedError, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedUser, user)
			}
		})
	}
}