package integration_tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-api-template/ent"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"          // For storage errors
	"go-api-template/internal/storage/postgres" // Need concrete repo for assertion
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9" // Import redis
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// --- Test Setup ---

const (
	testJwtSecret              = "test-integration-secret"
	testJwtExpiration          = 1 * time.Minute // Short duration for tests
	testRefreshTokenExpiration = 5 * time.Minute
)

// setupUserServiceIntegrationTest initializes the service with a real DB pool
// and potentially a real/mock Redis client.
func setupUserServiceIntegrationTest(t *testing.T) (context.Context, services.UserService, *ent.Client, *redis.Client) {
	t.Helper()
	pool, redisClient := getTestClients(t)
	userService := services.NewUserService(redisClient, testJwtSecret, testJwtExpiration, testRefreshTokenExpiration, pool)
	ctx := context.Background()
	return ctx, userService, pool, redisClient
}

// --- Test Cases ---

func TestUserService_Integration_RegisterAndGet(t *testing.T) {
	ctx, userService, pool, _ := setupUserServiceIntegrationTest(t)
	defer cleanupTables(t, pool, "users")

	// --- Register ---
	registerReq := &dto.CreateUserRequest{
		Email:    "register-get@test.com",
		Name:     "Register Get User",
		Password: "password123",
	}
	createdUser, err := userService.Register(ctx, registerReq)

	require.NoError(t, err)
	require.NotNil(t, createdUser)
	assert.Equal(t, registerReq.Email, createdUser.Email)
	assert.Equal(t, registerReq.Name, createdUser.Name)
	assert.NotEqual(t, uuid.Nil, createdUser.ID)

	// --- GetByID ---
	getByIDReq := &dto.GetUserByIdRequest{ID: createdUser.ID}
	fetchedUserByID, err := userService.GetByID(ctx, getByIDReq)

	require.NoError(t, err)
	require.NotNil(t, fetchedUserByID)
	assert.Equal(t, createdUser.ID, fetchedUserByID.ID)
	assert.Equal(t, createdUser.Email, fetchedUserByID.Email)
	assert.Equal(t, createdUser.Name, fetchedUserByID.Name)

	// --- GetByEmail ---
	getByEmailReq := &dto.GetUserByEmailRequest{Email: registerReq.Email}
	fetchedUserByEmail, err := userService.GetByEmail(ctx, getByEmailReq) // This fetches the hash too

	require.NoError(t, err)
	require.NotNil(t, fetchedUserByEmail)
	assert.Equal(t, createdUser.ID, fetchedUserByEmail.ID)
	assert.Equal(t, registerReq.Email, fetchedUserByEmail.Email)
	assert.Equal(t, registerReq.Name, fetchedUserByEmail.Name)
	// Verify password hash was stored and matches
	err = bcrypt.CompareHashAndPassword([]byte(fetchedUserByEmail.PasswordHash), []byte(registerReq.Password))
	assert.NoError(t, err, "Stored password hash should match original password")

	// --- GetByID - Not Found ---
	getByIDReqNotFound := &dto.GetUserByIdRequest{ID: uuid.New()}
	_, err = userService.GetByID(ctx, getByIDReqNotFound)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))

	// --- GetByEmail - Not Found ---
	getByEmailReqNotFound := &dto.GetUserByEmailRequest{Email: "notfound@test.com"}
	_, err = userService.GetByEmail(ctx, getByEmailReqNotFound)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))

	// --- Register - Duplicate Email ---
	duplicateRegisterReq := &dto.CreateUserRequest{
		Email:    "register-get@test.com", // Same email
		Name:     "Duplicate User",
		Password: "password456",
	}
	_, err = userService.Register(ctx, duplicateRegisterReq)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrConflict), "Expected ErrConflict, got %v", err) // Service maps storage.ErrDuplicateEmail
}

func TestUserService_Integration_Update(t *testing.T) {
	ctx, userService, pool, _ := setupUserServiceIntegrationTest(t)
	userRepo := postgres.NewUserRepo(pool) // For setup/verification
	defer cleanupTables(t, pool, "users")

	// --- Setup: Create a user ---
	initialEmail := "update-user@test.com"
	initialName := "Initial Name"
	registerReq := &dto.CreateUserRequest{
		Email:    initialEmail,
		Name:     initialName,
		Password: "password123",
	}
	createdUser, err := userRepo.Create(ctx, registerReq) // Use repo directly for setup
	require.NoError(t, err)

	// --- Test Execution: Update Name ---
	updatedName := "Updated Name"
	updateReq := &dto.UpdateUserRequest{
		ID:   createdUser.ID,
		Name: &updatedName,
	}
	updatedUser, err := userService.Update(ctx, updateReq)

	// --- Assertions ---
	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	assert.Equal(t, createdUser.ID, updatedUser.ID)
	assert.Equal(t, updatedName, updatedUser.Name)   // Check updated name
	assert.Equal(t, initialEmail, updatedUser.Email) // Email should not change
	assert.True(t, updatedUser.UpdatedAt.After(createdUser.UpdatedAt))

	// Verify directly in DB
	getReq := &dto.GetUserByIdRequest{ID: createdUser.ID}
	dbUser, dbErr := userRepo.GetByID(ctx, getReq)
	require.NoError(t, dbErr)
	assert.Equal(t, updatedName, dbUser.Name)
	assert.Equal(t, initialEmail, dbUser.Email)

	// --- Test Execution: Update Not Found ---
	updateReqNotFound := &dto.UpdateUserRequest{
		ID:   uuid.New(), // Non-existent ID
		Name: &updatedName,
	}
	_, err = userService.Update(ctx, updateReqNotFound)
	require.Error(t, err)
	// Check the specific error returned by the service after mapping
	assert.True(t, errors.Is(err, services.ErrNotFound), "Expected ErrNotFound, got %v", err)
}

func TestUserService_Integration_Delete(t *testing.T) {
	ctx, userService, pool, _ := setupUserServiceIntegrationTest(t)
	userRepo := postgres.NewUserRepo(pool) // For setup/verification
	defer cleanupTables(t, pool, "users")

	// --- Setup: Create a user ---
	registerReq := &dto.CreateUserRequest{
		Email:    "delete-user@test.com",
		Name:     "Delete Me",
		Password: "password123",
	}
	createdUser, err := userRepo.Create(ctx, registerReq) // Use repo directly for setup
	require.NoError(t, err)

	// --- Test Execution: Delete User ---
	deleteReq := &dto.DeleteUserRequest{ID: createdUser.ID}
	err = userService.Delete(ctx, deleteReq)

	// --- Assertions ---
	require.NoError(t, err)

	// Verify user is gone from DB
	getReq := &dto.GetUserByIdRequest{ID: createdUser.ID}
	_, dbErr := userRepo.GetByID(ctx, getReq)
	require.Error(t, dbErr)
	assert.True(t, errors.Is(dbErr, storage.ErrNotFound))

	// --- Test Execution: Delete Not Found ---
	deleteReqNotFound := &dto.DeleteUserRequest{ID: uuid.New()} // Non-existent ID
	err = userService.Delete(ctx, deleteReqNotFound)
	require.Error(t, err)
	// The service Delete just calls repo.Delete, which returns storage.ErrNotFound directly
	assert.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestUserService_Integration_GetAll(t *testing.T) {
	ctx, userService, pool, _ := setupUserServiceIntegrationTest(t)
	userRepo := postgres.NewUserRepo(pool) // For setup
	defer cleanupTables(t, pool, "users")

	// --- Setup: Create multiple users ---
	user1, err := userRepo.Create(ctx, &dto.CreateUserRequest{Email: "getall1@test.com", Name: "GetAll One", Password: "p"})
	require.NoError(t, err)
	user2, err := userRepo.Create(ctx, &dto.CreateUserRequest{Email: "getall2@test.com", Name: "GetAll Two", Password: "p"})
	require.NoError(t, err)

	// --- Test Execution ---
	users, err := userService.GetAll(ctx)

	// --- Assertions ---
	require.NoError(t, err)
	assert.Len(t, users, 2)
	// Optionally check specific fields if needed, keeping in mind order might vary
	// Use map for easier checking regardless of order
	userMap := make(map[uuid.UUID]*ent.User)
	for _, u := range users {
		userMap[u.ID] = u
	}
	assert.Contains(t, userMap, user1.ID)
	assert.Contains(t, userMap, user2.ID)
	assert.Equal(t, "getall1@test.com", userMap[user1.ID].Email)
	assert.Equal(t, "GetAll Two", userMap[user2.ID].Name)
}

// TestUserService_Integration_Login tests the login process including token generation and Redis storage.
func TestUserService_Integration_Login(t *testing.T) {
	ctx, userService, pool, redisClient := setupUserServiceIntegrationTest(t)
	if redisClient == nil {
		t.Skip("Skipping Redis test: TEST_REDIS_URL not set or connection failed")
	}
	userRepo := postgres.NewUserRepo(pool) // For setup
	defer cleanupTables(t, pool, "users")
	defer cleanupRedis(t, redisClient)

	// --- Setup: Create a user ---
	password := "loginPass123"
	user, err := userRepo.Create(ctx, &dto.CreateUserRequest{
		Email:    "login@test.com",
		Name:     "Login User",
		Password: password,
	})
	require.NoError(t, err)

	// --- Test Execution: Successful Login ---
	loginReq := &dto.LoginRequest{
		Email:    user.Email,
		Password: password,
	}
	loggedInUser, accessToken, refreshToken, err := userService.Login(ctx, loginReq)

	// --- Assertions ---
	require.NoError(t, err)
	require.NotNil(t, loggedInUser)
	assert.Equal(t, user.ID, loggedInUser.ID)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)

	// Verify refresh token exists in Redis
	redisKey := services.RedisRefreshTokenPrefix + refreshToken
	storedUserID, err := redisClient.Get(ctx, redisKey).Result()
	require.NoError(t, err, "Refresh token should exist in Redis")
	assert.Equal(t, user.ID.String(), storedUserID)

	// Verify TTL is set (approximately)
	ttl, err := redisClient.TTL(ctx, redisKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, testRefreshTokenExpiration) // Check it's not longer than expected

	// --- Test Execution: Invalid Password ---
	loginReqInvalidPass := &dto.LoginRequest{
		Email:    user.Email,
		Password: "wrongPassword",
	}
	_, _, _, err = userService.Login(ctx, loginReqInvalidPass)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))

	// --- Test Execution: User Not Found ---
	loginReqNotFound := &dto.LoginRequest{
		Email:    "nosuchuser@test.com",
		Password: password,
	}
	_, _, _, err = userService.Login(ctx, loginReqNotFound)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials)) // Service maps NotFound to InvalidCredentials
}

// TestUserService_Integration_Refresh tests token refresh using Redis.
func TestUserService_Integration_Refresh(t *testing.T) {
	ctx, userService, pool, redisClient := setupUserServiceIntegrationTest(t)
	if redisClient == nil {
		t.Skip("Skipping Redis test: TEST_REDIS_URL not set or connection failed")
	}
	userRepo := postgres.NewUserRepo(pool) // For setup
	defer cleanupTables(t, pool, "users")
	defer cleanupRedis(t, redisClient)

	// --- Setup: Create user and perform initial login to get a refresh token ---
	user, err := userRepo.Create(ctx, &dto.CreateUserRequest{Email: "refresh@test.com", Name: "Refresh User", Password: "p"})
	require.NoError(t, err)
	_, _, initialRefreshToken, err := userService.Login(ctx, &dto.LoginRequest{Email: user.Email, Password: "p"})
	require.NoError(t, err)
	require.NotEmpty(t, initialRefreshToken)

	// --- Test Execution: Successful Refresh ---
	refreshReq := &dto.RefreshRequest{RefreshToken: initialRefreshToken}
	newAccessToken, newRefreshToken, err := userService.Refresh(ctx, refreshReq)

	// --- Assertions ---
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEmpty(t, newRefreshToken)
	assert.NotEqual(t, initialRefreshToken, newRefreshToken, "A new refresh token should be issued")

	// Verify old refresh token is deleted from Redis
	oldRedisKey := services.RedisRefreshTokenPrefix + initialRefreshToken
	err = redisClient.Get(ctx, oldRedisKey).Err()
	require.Error(t, err)
	assert.True(t, errors.Is(err, redis.Nil), "Old refresh token should be deleted")

	// Verify new refresh token exists in Redis
	newRedisKey := services.RedisRefreshTokenPrefix + newRefreshToken
	storedUserID, err := redisClient.Get(ctx, newRedisKey).Result()
	require.NoError(t, err, "New refresh token should exist in Redis")
	assert.Equal(t, user.ID.String(), storedUserID)

	// --- Test Execution: Invalid Refresh Token ---
	refreshReqInvalid := &dto.RefreshRequest{RefreshToken: "invalid-token"}
	_, _, err = userService.Refresh(ctx, refreshReqInvalid)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))

	// --- Test Execution: Used Refresh Token ---
	refreshReqUsed := &dto.RefreshRequest{RefreshToken: initialRefreshToken} // Try using the old one again
	_, _, err = userService.Refresh(ctx, refreshReqUsed)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidCredentials))
}

// TestUserService_Integration_Logout tests invalidating refresh tokens via Redis.
func TestUserService_Integration_Logout(t *testing.T) {
	ctx, userService, pool, redisClient := setupUserServiceIntegrationTest(t)
	if redisClient == nil {
		t.Skip("Skipping Redis test: TEST_REDIS_URL not set or connection failed")
	}
	userRepo := postgres.NewUserRepo(pool) // For setup
	defer cleanupTables(t, pool, "users")
	defer cleanupRedis(t, redisClient)

	// --- Setup: Create user and login ---
	user, err := userRepo.Create(ctx, &dto.CreateUserRequest{Email: "logout@test.com", Name: "Logout User", Password: "p"})
	require.NoError(t, err)
	_, _, refreshToken, err := userService.Login(ctx, &dto.LoginRequest{Email: user.Email, Password: "p"})
	require.NoError(t, err)
	require.NotEmpty(t, refreshToken)

	// --- Test Execution: Successful Logout ---
	logoutReq := &dto.LogoutRequest{RefreshToken: refreshToken}
	err = userService.Logout(ctx, logoutReq)

	// --- Assertions ---
	require.NoError(t, err)

	// Verify refresh token is deleted from Redis
	redisKey := services.RedisRefreshTokenPrefix + refreshToken
	err = redisClient.Get(ctx, redisKey).Err()
	require.Error(t, err)
	assert.True(t, errors.Is(err, redis.Nil), "Refresh token should be deleted after logout")

	// --- Test Execution: Logout with Invalid/Used Token (should not error) ---
	logoutReqInvalid := &dto.LogoutRequest{RefreshToken: "invalid-token"}
	err = userService.Logout(ctx, logoutReqInvalid)
	require.NoError(t, err, "Logout with non-existent token should not return an error")

	logoutReqUsed := &dto.LogoutRequest{RefreshToken: refreshToken} // Use the already logged out token
	err = userService.Logout(ctx, logoutReqUsed)
	require.NoError(t, err, "Logout with already invalidated token should not return an error")
}
