package integration_tests

import (
	"context" // Needed for JWT secret env var
	"testing"
	"time"

	// Should be imported correctly
	"go-api-template/internal/api"
	"go-api-template/internal/services" // Import the services package

	// Import storage errors
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9" // Use v9 for compatibility with the helper
	"github.com/stretchr/testify/require"
)

// setupUserService initializes a new ServerDefinition for testing and cleans up tables/redis.
func setupUserService(t *testing.T) *services.ServerDefinition {
	t.Helper()

	getTestClients(t)

	// Clean up database and redis state before each test
	ctx := context.Background()
	cleanupTables(ctx, t, testDB, "users", "jobs", "invoices", "job_application") // Clean all tables potentially affected
	cleanupRedis(t, testRedisClient)

	// Get JWT Secret from environment variable
	jwtSecret := "test-jwt-secret"

	return services.NewServerDefinition(
		testDB,          // Use the package-level DB client from getTestClients
		testRedisClient, // Use the package-level Redis client
		jwtSecret,
		15*time.Minute, // Short JWT expiration for tests
		24*time.Hour,   // Longer refresh token expiration
	)
}

func TestUserService_PostAuthRegister(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// --- Test Case 1: Successful Registration ---
	t.Run("SuccessfulRegistration", func(t *testing.T) {
		name := "Test User"
		email := "test.register@example.com"
		req := api.PostAuthRegisterRequestObject{
			Body: &api.PostAuthRegisterJSONRequestBody{
				Email:    email,
				Password: "password123",
				Name:     &name,
			},
		}

		resp, err := sd.PostAuthRegister(ctx, req)
		require.NoError(t, err, "registration should succeed")

		registerResp, ok := resp.(api.PostAuthRegister201JSONResponse)
		require.True(t, ok, "response should be 201 JSON")
		require.NotNil(t, registerResp, "response body should not be nil")
		require.NotNil(t, registerResp.Id, "user ID should not be nil")
		require.Equal(t, req.Body.Email, *registerResp.Email, "emails should match")
		require.Equal(t, *req.Body.Name, *registerResp.Name, "names should match")

		// Verify user exists in DB
		user, err := testDB.User.Get(ctx, uuid.MustParse(*registerResp.Id))
		require.NoError(t, err, "user should exist in database after registration")
		require.Equal(t, req.Body.Email, user.Email)
	})

	// --- Test Case 2: Registration with Duplicate Email ---
	t.Run("DuplicateEmail", func(t *testing.T) {
		name := "Existing User"
		existingEmail := "existing.user@example.com"

		// Setup: Create the user using the helper
		createTestUser(t, ctx, testDB, existingEmail, name)

		// Attempt to register with the same email again
		req := api.PostAuthRegisterRequestObject{
			Body: &api.PostAuthRegisterJSONRequestBody{
				Email:    existingEmail,
				Password: "anotherpassword",
				Name:     &name,
			},
		}

		_, err := sd.PostAuthRegister(ctx, req)
		require.Error(t, err, "registration with duplicate email should fail")
		// Use errors.Is for checking specific service errors
		require.ErrorIs(t, err, services.ErrConflict, "error should be services.ErrConflict")
	})
}

func TestUserService_PostAuthLogin(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user first using the helper
	name := "Login User"
	email := "test.login@example.com"
	password := "password"
	registeredUser := createTestUser(t, ctx, testDB, email, name)

	// --- Test Case 1: Successful Login ---
	t.Run("SuccessfulLogin", func(t *testing.T) {
		req := api.PostAuthLoginRequestObject{
			Body: &api.DtoLoginRequest{
				Email:    email,
				Password: password,
			},
		}

		resp, err := sd.PostAuthLogin(ctx, req)
		require.NoError(t, err, "login should succeed")

		loginResp, ok := resp.(api.PostAuthLogin200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, loginResp, "response body should not be nil")
		require.NotNil(t, loginResp.AccessToken, "access token should not be nil")
		require.NotNil(t, loginResp.RefreshToken, "refresh token should not be nil")
		require.NotNil(t, loginResp.User, "user object should not be nil")
		require.Equal(t, registeredUser.ID.String(), *loginResp.User.Id, "logged in user ID should match registered ID")

		// Verify refresh token is stored in Redis
		redisKey := "refresh_token:" + *loginResp.RefreshToken
		userIDInRedis, err := testRedisClient.Get(ctx, redisKey).Result()
		require.NoError(t, err, "refresh token should be stored in redis")
		require.Equal(t, registeredUser.ID.String(), userIDInRedis, "user ID stored in redis should match registered ID")
	})

	// --- Test Case 2: Login with Incorrect Password ---
	t.Run("IncorrectPassword", func(t *testing.T) {
		req := api.PostAuthLoginRequestObject{
			Body: &api.DtoLoginRequest{
				Email:    email,
				Password: "wrongpassword",
			},
		}

		_, err := sd.PostAuthLogin(ctx, req)
		require.Error(t, err, "login with incorrect password should fail")
		require.ErrorIs(t, err, services.ErrInvalidCredentials, "error should be services.ErrInvalidCredentials")
	})

	// --- Test Case 3: Login with Non-existent Email ---
	t.Run("NonExistentEmail", func(t *testing.T) {
		req := api.PostAuthLoginRequestObject{
			Body: &api.DtoLoginRequest{
				Email:    "nonexistent@example.com",
				Password: "anypassword",
			},
		}

		_, err := sd.PostAuthLogin(ctx, req)
		require.Error(t, err, "login with non-existent email should fail")
		require.ErrorIs(t, err, services.ErrInvalidCredentials, "error should be services.ErrInvalidCredentials")
	})
}

func TestUserService_PostAuthRefresh(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user and log in to get a refresh token
	name := "Refresh User"
	email := "test.refresh@example.com"
	password := "password"
	registeredUser := createTestUser(t, ctx, testDB, email, name)

	loginReq := api.PostAuthLoginRequestObject{Body: &api.DtoLoginRequest{Email: email, Password: password}}
	loginResp, err := sd.PostAuthLogin(ctx, loginReq)
	require.NoError(t, err, "setup: login should succeed")
	initialLoginResp := loginResp.(api.PostAuthLogin200JSONResponse)
	initialRefreshToken := *initialLoginResp.RefreshToken
	initialAccessToken := *initialLoginResp.AccessToken
	time.Sleep(1 * time.Second)

	// --- Test Case 1: Successful Token Refresh ---
	t.Run("SuccessfulRefresh", func(t *testing.T) {
		req := api.PostAuthRefreshRequestObject{
			Body: &api.DtoRefreshRequest{
				RefreshToken: initialRefreshToken,
			},
		}

		resp, err := sd.PostAuthRefresh(ctx, req)
		require.NoError(t, err, "token refresh should succeed")

		refreshResp, ok := resp.(api.PostAuthRefresh200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, refreshResp, "response body should not be nil")
		require.NotNil(t, refreshResp.AccessToken, "new access token should not be nil")
		require.NotNil(t, refreshResp.RefreshToken, "new refresh token should not be nil")

		require.NotEqual(t, initialAccessToken, *refreshResp.AccessToken, "new access token should be different")
		require.NotEqual(t, initialRefreshToken, *refreshResp.RefreshToken, "new refresh token should be different")

		// Verify initial refresh token is deleted from Redis
		initialRedisKey := "refresh_token:" + initialRefreshToken
		_, err = testRedisClient.Get(ctx, initialRedisKey).Result()
		require.ErrorIs(t, err, redis.Nil, "initial refresh token should be deleted from redis")

		// Verify new refresh token is stored in Redis
		newRedisKey := "refresh_token:" + *refreshResp.RefreshToken
		userIDInRedis, err := testRedisClient.Get(ctx, newRedisKey).Result()
		require.NoError(t, err, "new refresh token should be stored in redis")
		require.Equal(t, registeredUser.ID.String(), userIDInRedis, "user ID stored with new refresh token should match")
	})

	// --- Test Case 2: Refresh with Invalid/Expired Token ---
	t.Run("InvalidOrExpiredToken", func(t *testing.T) {
		req := api.PostAuthRefreshRequestObject{
			Body: &api.DtoRefreshRequest{
				RefreshToken: "invalid_token_123",
			},
		}

		_, err := sd.PostAuthRefresh(ctx, req)
		require.Error(t, err, "refresh with invalid token should fail")
		require.ErrorContains(t, err, "internal error validating refresh token", "error should contain")
	})
}

func TestUserService_PostAuthLogout(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user and log in to get a refresh token
	name := "Logout User"
	email := "test.logout@example.com"
	password := "password"
	createTestUser(t, ctx, testDB, email, name)

	loginReq := api.PostAuthLoginRequestObject{Body: &api.DtoLoginRequest{Email: email, Password: password}}
	loginResp, err := sd.PostAuthLogin(ctx, loginReq)
	require.NoError(t, err, "setup: login should succeed")
	initialLoginResp := loginResp.(api.PostAuthLogin200JSONResponse)
	refreshTokenToLogout := *initialLoginResp.RefreshToken

	// Verify refresh token exists in Redis initially
	redisKey := "refresh_token:" + refreshTokenToLogout
	_, err = testRedisClient.Get(ctx, redisKey).Result()
	require.NoError(t, err, "setup: refresh token should exist in redis before logout")

	// --- Test Case 1: Successful Logout ---
	t.Run("SuccessfulLogout", func(t *testing.T) {
		req := api.PostAuthLogoutRequestObject{
			Body: &api.PostAuthLogoutJSONRequestBody{
				RefreshToken: refreshTokenToLogout,
			},
		}

		resp, err := sd.PostAuthLogout(ctx, req)
		require.NoError(t, err, "logout should succeed")

		_, ok := resp.(api.PostAuthLogout204Response)
		require.True(t, ok, "response should be 204 No Content")

		// Verify refresh token is deleted from Redis
		_, err = testRedisClient.Get(ctx, redisKey).Result()
		require.ErrorIs(t, err, redis.Nil, "refresh token should be deleted from redis after logout")
	})

	// --- Test Case 2: Logout with Non-existent Token ---
	t.Run("NonExistentToken", func(t *testing.T) {
		// Cleanup the valid token from setup first
		cleanupRedis(t, testRedisClient)

		req := api.PostAuthLogoutRequestObject{
			Body: &api.PostAuthLogoutJSONRequestBody{
				RefreshToken: "already_logged_out_token_456",
			},
		}

		// Logout should ideally succeed or not return an error even if the token doesn't exist
		// based on the current implementation which ignores redis.Nil error.
		_, err := sd.PostAuthLogout(ctx, req)
		require.NoError(t, err, "logout with non-existent token should not return an error")
	})
}

func TestUserService_GetUsersId(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user using the helper
	name := "Lookup User"
	email := "test.lookup@example.com"
	registeredUser := createTestUser(t, ctx, testDB, email, name)
	registeredUserID := registeredUser.ID
	registeredUserIDStr := registeredUserID.String()

	// --- Test Case 1: Get Existing User by ID ---
	t.Run("GetExistingUser", func(t *testing.T) {
		req := api.GetUsersIdRequestObject{Id: registeredUserID}

		resp, err := sd.GetUsersId(ctx, req)
		require.NoError(t, err, "getting existing user by ID should succeed")

		getUserResp, ok := resp.(api.GetUsersId200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, getUserResp, "response body should not be nil")
		require.Equal(t, registeredUserIDStr, *getUserResp.Id, "returned user ID should match requested ID")
		require.Equal(t, email, *getUserResp.Email)
		require.Equal(t, name, *getUserResp.Name)
	})

	// --- Test Case 2: Get Non-existent User by ID ---
	t.Run("GetNonExistentUser", func(t *testing.T) {
		nonExistentUserID := uuid.New()
		req := api.GetUsersIdRequestObject{Id: nonExistentUserID}

		_, err := sd.GetUsersId(ctx, req)
		require.Error(t, err, "getting non-existent user by ID should fail")
		// Use errors.Is for checking specific service errors
		require.ErrorIs(t, err, services.ErrNotFound, "error should be services.ErrNotFound")
	})
}

func TestUserService_PutUsersId(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user using the helper
	name := "Initial Name"
	email := "test.update@example.com"
	registeredUser := createTestUser(t, ctx, testDB, email, name)
	registeredUserID := registeredUser.ID
	registeredUserIDStr := registeredUserID.String()

	// --- Test Case 1: Successfully Update User Name ---
	t.Run("SuccessfulUpdate", func(t *testing.T) {
		newName := "Updated Name"
		req := api.PutUsersIdRequestObject{
			Id: registeredUserID,
			Body: &api.DtoUpdateUserRequest{
				Name: &newName,
			},
		}

		resp, err := sd.PutUsersId(ctx, req)
		require.NoError(t, err, "updating user should succeed")

		updateUserResp, ok := resp.(api.PutUsersId200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, updateUserResp, "response body should not be nil")
		require.Equal(t, registeredUserIDStr, *updateUserResp.Id, "returned user ID should match updated ID")
		require.Equal(t, newName, *updateUserResp.Name, "user name should be updated")
		require.Equal(t, email, *updateUserResp.Email, "user email should not change") // Assuming email is not updatable

		// Verify user name is updated in the database
		userInDB, err := testDB.User.Get(ctx, registeredUserID)
		require.NoError(t, err, "failed to retrieve user from DB after update")
		require.Equal(t, newName, userInDB.Name, "user name in DB should be updated")
	})

	// --- Test Case 2: Attempt to Update Non-existent User ---
	t.Run("UpdateNonExistentUser", func(t *testing.T) {
		nonExistentUserID := uuid.New()
		newName := "Should Not Exist"
		req := api.PutUsersIdRequestObject{
			Id: nonExistentUserID,
			Body: &api.DtoUpdateUserRequest{
				Name: &newName,
			},
		}

		_, err := sd.PutUsersId(ctx, req)
		require.Error(t, err, "updating non-existent user should fail")
		// Use errors.Is for checking specific service errors
		require.ErrorIs(t, err, services.ErrNotFound, "error should be services.ErrNotFound") // Assuming mapRepoError maps storage.ErrNotFound to services.ErrNotFound
	})
}

func TestUserService_GetUsers(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// --- Test Case 1: Get Users when none exist ---
	t.Run("GetUsersEmpty", func(t *testing.T) {
		req := api.GetUsersRequestObject{}
		resp, err := sd.GetUsers(ctx, req)
		require.NoError(t, err, "getting users when empty should succeed")

		getUsersResp, ok := resp.(api.GetUsers200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, getUsersResp, "response body should not be nil")
		require.Len(t, getUsersResp, 0, "should return an empty list when no users exist")
	})

	// --- Test Case 2: Get Users when some exist ---
	t.Run("GetUsersPopulated", func(t *testing.T) {
		// Setup: Register a couple of users using the helper
		user1Name := "User One"
		user1Email := "user.one@example.com"
		user2Name := "User Two"
		user2Email := "user.two@example.com"

		user1 := createTestUser(t, ctx, testDB, user1Email, user1Name)
		user2 := createTestUser(t, ctx, testDB, user2Email, user2Name)

		// Now get all users
		req := api.GetUsersRequestObject{}
		resp, err := sd.GetUsers(ctx, req)
		require.NoError(t, err, "getting users should succeed")

		getUsersResp, ok := resp.(api.GetUsers200JSONResponse)
		require.True(t, ok, "response should be 200 JSON")
		require.NotNil(t, getUsersResp, "response body should not be nil")
		require.Len(t, getUsersResp, 2, "should return 2 users")

		// Verify the returned users (order might not be guaranteed, so check for presence and data)
		foundUser1 := false
		foundUser2 := false
		for _, user := range getUsersResp {
			if *user.Email == user1Email {
				foundUser1 = true
				require.Equal(t, user1Name, *user.Name)
				require.Equal(t, user1.ID.String(), *user.Id)
			}
			if *user.Email == user2Email {
				foundUser2 = true
				require.Equal(t, user2Name, *user.Name)
				require.Equal(t, user2.ID.String(), *user.Id)
			}
		}
		require.True(t, foundUser1, "should find user 1 in the list")
		require.True(t, foundUser2, "should find user 2 in the list")
	})
}

func TestUserService_DeleteUsersId(t *testing.T) {
	sd := setupUserService(t) // Use the helper setup
	ctx := context.Background()

	// Setup: Register a user to be deleted using the helper
	name := "Delete User"
	email := "test.delete@example.com"
	registeredUser := createTestUser(t, ctx, testDB, email, name)
	registeredUserID := registeredUser.ID

	// Verify user exists in DB before deletion
	_, err := testDB.User.Get(ctx, registeredUserID)
	require.NoError(t, err, "setup: user should exist in database before deletion")

	// --- Test Case 1: Successfully Delete User by ID ---
	t.Run("SuccessfulDelete", func(t *testing.T) {
		req := api.DeleteUsersIdRequestObject{Id: registeredUserID}

		resp, err := sd.DeleteUsersId(ctx, req)
		require.NoError(t, err, "deleting user should succeed")

		_, ok := resp.(api.DeleteUsersId204Response)
		require.True(t, ok, "response should be 204 No Content")

		// Verify user is deleted from the database
		_, err = testDB.User.Get(ctx, registeredUserID)
		require.Error(t, err, "user should not exist in database after deletion")
	})

	// --- Test Case 2: Attempt to Delete Non-existent User ---
	t.Run("DeleteNonExistentUser", func(t *testing.T) {
		nonExistentUserID := uuid.New()
		req := api.DeleteUsersIdRequestObject{Id: nonExistentUserID}

		_, err := sd.DeleteUsersId(ctx, req)
		require.Error(t, err, "resource not found")
	})
}
