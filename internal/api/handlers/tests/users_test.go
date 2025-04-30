package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go-api-template/internal/api/handlers"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/api/routes"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// MockUserHandler for route registration test ONLY
type MockRouteTestUserHandler struct {
	mock.Mock
}
func (m *MockRouteTestUserHandler) GetUserByID(c *gin.Context) { m.Called(c) }
func (m *MockRouteTestUserHandler) GetUsers(c *gin.Context)    { m.Called(c) }
func (m *MockRouteTestUserHandler) Register(c *gin.Context)    { m.Called(c) }
func (m *MockRouteTestUserHandler) Login(c *gin.Context)       { m.Called(c) }
func (m *MockRouteTestUserHandler) UpdateUser(c *gin.Context)  { m.Called(c) }
func (m *MockRouteTestUserHandler) DeleteUser(c *gin.Context)  { m.Called(c) }

// Ensure mock implements the interface
var _ handlers.UserHandlerInterface = (*MockRouteTestUserHandler)(nil)

// MockUserRepository is a mock type for the storage.UserRepository interface
type MockUserRepository struct {
	mock.Mock
}

// Implement methods used by the handler
func (m *MockUserRepository) GetAll(ctx context.Context) ([]models.User, error) {
	args := m.Called(ctx)
	// Type assertion needed for the slice
	if users, ok := args.Get(0).([]models.User); ok {
		return users, args.Error(1)
	}
	// Handle case where nil is returned for the slice explicitly
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	// Fallback or error if type assertion fails unexpectedly
	return nil, errors.New("mock return value type mismatch for []models.User")

}

func (m *MockUserRepository) GetByEmail(ctx context.Context, req *dto.GetUserByEmailRequest) (*models.User, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, req *dto.GetUserByIdRequest) (*models.User, error) {
	args := m.Called(ctx, req)
	// Handle nil return for pointer type
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Create(ctx context.Context, userReq *dto.CreateUserRequest) (*models.User, error) {
	args := m.Called(ctx, userReq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, userReq *dto.UpdateUserRequest) (*models.User, error) {
	args := m.Called(ctx, userReq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Delete(ctx context.Context, userReq *dto.DeleteUserRequest) error {
	args := m.Called(ctx, userReq)
	return args.Error(0)
}

// Ensure mock implements the interface
var _ storage.UserRepository = (*MockUserRepository)(nil)

// --- Helper Function for Setup ---

const testJWTSecret = "test-secret-key-for-unit-tests"
var testJWTExpiration = 15 * time.Minute

func setupTestRouterWithUserMocks() (*gin.Engine, *MockUserRepository, *handlers.UserHandler) {
	gin.SetMode(gin.TestMode)
	mockRepo := new(MockUserRepository)
	validate := validator.New()
	handler := handlers.NewUserHandler(mockRepo, validate, testJWTSecret, testJWTExpiration)
	router := gin.New()
	return router, mockRepo, handler
}


func TestRegisterUserRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode) // Set Gin to test mode

	mockHandler := new(MockRouteTestUserHandler) // Create instance of the mock handler

	router := gin.New()              // Create a new Gin engine for testing
	testGroup := router.Group("/api/v1") // Create a base group similar to potential real setup

	testAuthMiddleware := middleware.JWTAuthMiddleware(testJWTSecret) // Mock JWT middleware
	// Act
	routes.RegisterUserRoutes(testGroup, mockHandler, testAuthMiddleware) // Call the function under test

	// Assert
	// Check if the expected routes are registered
	expectedRoutes := []struct {
		Method string
		Path   string
	}{
		// Auth routes
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodPost, "/api/v1/auth/login"},
		// User CRUD routes
		{http.MethodGet, "/api/v1/users/"},
		{http.MethodGet, "/api/v1/users/:id"},
		{http.MethodPut, "/api/v1/users/:id"},
		{http.MethodDelete, "/api/v1/users/:id"},
	}

	registeredRoutes := router.Routes()

	// Build a map for quick lookup of registered routes
	registeredMap := make(map[string]bool)
	for _, routeInfo := range registeredRoutes {
		mapKey := routeInfo.Method + " " + routeInfo.Path
		registeredMap[mapKey] = true
		t.Logf("Registered: %s %s", routeInfo.Method, routeInfo.Path) // Log registered routes for debugging
	}

	// Check if all expected routes exist
	assert.Len(t, registeredRoutes, len(expectedRoutes), "Number of registered routes should match expected")

	for _, expected := range expectedRoutes {
		mapKey := expected.Method + " " + expected.Path
		assert.True(t, registeredMap[mapKey], "Expected route %s %s to be registered", expected.Method, expected.Path)
	}
}

// --- Handler Tests ---

func TestUserHandler_Register(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/register", handler.Register)

		reqBody := dto.CreateUserRequest{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "password123",
		}
		mockUserID := uuid.New()
		now := time.Now()
		mockUser := &models.User{
			ID:           mockUserID,
			Name:         reqBody.Name,
			Email:        reqBody.Email,
			PasswordHash: "hashed_password_ignored_in_response",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		mockRepo.On("Create", mock.Anything, &reqBody).Return(mockUser, nil).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusCreated, recorder.Code)
		var resp dto.UserResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, mockUserID, resp.ID)
		assert.Equal(t, reqBody.Name, resp.Name)
		assert.Equal(t, reqBody.Email, resp.Email)
		assert.WithinDuration(t, now, resp.CreatedAt, time.Second)
		assert.WithinDuration(t, now, resp.UpdatedAt, time.Second)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/register", handler.Register)

		reqBody := dto.CreateUserRequest{ Name: "Test" } // Missing required fields

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Validation failed")
		assert.Contains(t, recorder.Body.String(), "Email")
		assert.Contains(t, recorder.Body.String(), "Password")
		mockRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Duplicate Email Error", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/register", handler.Register)

		reqBody := dto.CreateUserRequest{
			Name:     "Test User",
			Email:    "duplicate@example.com",
			Password: "password123",
		}
		mockRepo.On("Create", mock.Anything, &reqBody).Return(nil, storage.ErrDuplicateEmail).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusConflict, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Email address already registered")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Internal Server Error on Create", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/register", handler.Register)

		reqBody := dto.CreateUserRequest{
			Name:     "Test User",
			Email:    "test-fail@example.com",
			Password: "password123",
		}
		mockRepo.On("Create", mock.Anything, &reqBody).Return(nil, errors.New("database exploded")).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Failed to register user")
		mockRepo.AssertExpectations(t)
	})
}

func TestUserHandler_Login(t *testing.T) {
	testEmail := "login@example.com"
	testPassword := "password123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	mockUserID := uuid.New()
	now := time.Now()
	mockUser := &models.User{
		ID:           mockUserID,
		Name:         "Login User",
		Email:        testEmail,
		PasswordHash: string(hashedPassword),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	t.Run("Success", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/login", handler.Login)

		reqBody := dto.LoginRequest{ Email: testEmail, Password: testPassword }
		emailReq := dto.GetUserByEmailRequest{Email: testEmail}
		mockRepo.On("GetByEmail", mock.Anything, &emailReq).Return(mockUser, nil).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var resp dto.LoginResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, mockUserID, resp.User.ID)
		assert.Equal(t, testEmail, resp.User.Email)
		// Assert token is present and non-empty
		assert.NotEmpty(t, resp.Token, "Token should not be empty on successful login")

		token, err := jwt.ParseWithClaims(resp.Token, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(testJWTSecret), nil // Use the same secret used in setup
		})
		assert.NoError(t, err, "Token should be parseable")
		assert.True(t, token.Valid, "Token should be valid")
		if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
			assert.Equal(t, mockUserID.String(), claims.Subject, "Token subject should match user ID")
			assert.WithinDuration(t, time.Now().Add(testJWTExpiration), claims.ExpiresAt.Time, 5*time.Second, "Token expiration time is incorrect")
		} else {
			t.Errorf("Could not parse token claims")
		}

		mockRepo.AssertExpectations(t)
	})

	t.Run("User Not Found", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/login", handler.Login)

		reqBody := dto.LoginRequest{ Email: "notfound@example.com", Password: testPassword }
		emailReq := dto.GetUserByEmailRequest{Email: "notfound@example.com"}
		mockRepo.On("GetByEmail", mock.Anything, &emailReq).Return(nil, storage.ErrNotFound).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Invalid email or password")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Incorrect Password", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/login", handler.Login)

		reqBody := dto.LoginRequest{ Email: testEmail, Password: "wrongpassword" }
		emailReq := dto.GetUserByEmailRequest{Email: testEmail}
		mockRepo.On("GetByEmail", mock.Anything, &emailReq).Return(mockUser, nil).Once() // Still returns user

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Invalid email or password")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Validation Error", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/login", handler.Login)

		reqBody := dto.LoginRequest{ Email: "invalid-email", Password: testPassword }

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Validation failed")
		assert.Contains(t, recorder.Body.String(), "Email")
		mockRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Internal Server Error on GetByEmail", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.POST("/auth/login", handler.Login)

		reqBody := dto.LoginRequest{ Email: testEmail, Password: testPassword }
		emailReq := dto.GetUserByEmailRequest{Email: testEmail}
		mockRepo.On("GetByEmail", mock.Anything, &emailReq).Return(nil, errors.New("db connection lost")).Once()

		bodyBytes, _ := json.Marshal(reqBody)
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(bodyBytes))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Login failed")
		mockRepo.AssertExpectations(t)
	})
}

func TestUserHandler_GetUsers(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users", handler.GetUsers)

		now := time.Now()
		mockUsers := []models.User{ // Data returned by mock repo
			{ID: uuid.New(), Name: "User 1", Email: "user1@example.com", CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), Name: "User 2", Email: "user2@example.com", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		}
		// Define expected DTOs based on mock data
		expectedResponses := []dto.UserResponse{
			handlers.MapUserModelToUserResponse(&mockUsers[0]),
			handlers.MapUserModelToUserResponse(&mockUsers[1]),
		}
		mockRepo.On("GetAll", mock.Anything).Return(mockUsers, nil).Once()

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var actualResponses []dto.UserResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &actualResponses)
		assert.NoError(t, err)

		// --- Compare slices element by element ---
		assert.Len(t, actualResponses, len(expectedResponses), "Number of users should match")

		// If the order is guaranteed (which it should be based on the mock), compare element by element
		for i := range expectedResponses {
			if i >= len(actualResponses) {
				t.Errorf("Actual responses slice is shorter than expected")
				break // Avoid index out of range
			}
			assert.Equal(t, expectedResponses[i].ID, actualResponses[i].ID, "User ID mismatch at index %d", i)
			assert.Equal(t, expectedResponses[i].Name, actualResponses[i].Name, "User Name mismatch at index %d", i)
			assert.Equal(t, expectedResponses[i].Email, actualResponses[i].Email, "User Email mismatch at index %d", i)
			// Use WithinDuration for time fields
			assert.WithinDuration(t, expectedResponses[i].CreatedAt, actualResponses[i].CreatedAt, time.Second, "User CreatedAt mismatch at index %d", i)
			assert.WithinDuration(t, expectedResponses[i].UpdatedAt, actualResponses[i].UpdatedAt, time.Second, "User UpdatedAt mismatch at index %d", i)
		}
		// -----------------------------------------

		mockRepo.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users", handler.GetUsers)

		mockRepo.On("GetAll", mock.Anything).Return(nil, errors.New("db error")).Once()

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Failed to retrieve users")
		mockRepo.AssertExpectations(t)
	})
}

func TestUserHandler_GetUserByID(t *testing.T) {
	testID := uuid.New()
	now := time.Now()
	mockUser := &models.User{
		ID:           testID,
		Name:         "Get User",
		Email:        "get@example.com",
		PasswordHash: "somehash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	t.Run("Success", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users/:id", handler.GetUserByID)

		idReq := &dto.GetUserByIdRequest{ID: testID}
		mockRepo.On("GetByID", mock.Anything, idReq).Return(mockUser, nil).Once()

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusOK, recorder.Code)
		var resp dto.UserResponse
		err := json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, testID, resp.ID)
		assert.Equal(t, mockUser.Name, resp.Name)
		assert.Equal(t, mockUser.Email, resp.Email)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users/:id", handler.GetUserByID)

		notFoundID := uuid.New()
		idReq := &dto.GetUserByIdRequest{ID: notFoundID}
		mockRepo.On("GetByID", mock.Anything, idReq).Return(nil, storage.ErrNotFound).Once()

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+notFoundID.String(), nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "User not found")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid ID Format", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users/:id", handler.GetUserByID)

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/invalid-uuid", nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Invalid user ID format")
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Internal Server Error", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		router.GET("/users/:id", handler.GetUserByID)

		errorID := uuid.New()
		idReq := &dto.GetUserByIdRequest{ID: errorID}
		mockRepo.On("GetByID", mock.Anything, idReq).Return(nil, errors.New("db error")).Once()

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+errorID.String(), nil)
		router.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Failed to retrieve user")
		mockRepo.AssertExpectations(t)
	})
}

func TestUserHandler_GetUserByID_Protected(t *testing.T) {
	testID := uuid.New() // User being requested
	authUserID := uuid.New() // User making the request (authenticated)
	now := time.Now()
	mockUser := &models.User{
		ID:           testID,
		Name:         "Get User",
		Email:        "get@example.com",
		PasswordHash: "somehash",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	authMiddleware := middleware.JWTAuthMiddleware(testJWTSecret)

	t.Run("Success with Valid Token", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		userGroup := router.Group("/users")
		userGroup.Use(authMiddleware)       
		userGroup.GET("/:id", handler.GetUserByID)
		idReq := &dto.GetUserByIdRequest{ID: testID}
		mockRepo.On("GetByID", mock.Anything, idReq).Return(mockUser, nil).Once()

		// Generate a valid token for authUserID
		validToken, err := generateTestToken(authUserID, testJWTSecret, testJWTExpiration)
		assert.NoError(t, err)

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		request.Header.Set("Authorization", "Bearer "+validToken) // Set Auth header
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)
		var resp dto.UserResponse
		err = json.Unmarshal(recorder.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, testID, resp.ID)
		assert.Equal(t, mockUser.Name, resp.Name)
		assert.Equal(t, mockUser.Email, resp.Email)
		mockRepo.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Unauthorized - No Token", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		userGroup := router.Group("/users")
		userGroup.Use(authMiddleware)       
		userGroup.GET("/:id", handler.GetUserByID)
		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		// No Authorization header
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Authorization header required")
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("Unauthorized - Invalid Token Format", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		userGroup := router.Group("/users")
		userGroup.Use(authMiddleware)       
		userGroup.GET("/:id", handler.GetUserByID)
		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		request.Header.Set("Authorization", "BearerTokenWithoutSpace") // Invalid format
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Invalid Authorization header format")
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("Unauthorized - Invalid Token Signature", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		userGroup := router.Group("/users")
		userGroup.Use(authMiddleware)       
		userGroup.GET("/:id", handler.GetUserByID)
		// Generate token with a DIFFERENT secret
		invalidToken, err := generateTestToken(authUserID, "wrong-secret", testJWTExpiration)
		assert.NoError(t, err)

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		request.Header.Set("Authorization", "Bearer "+invalidToken)
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Invalid token") // Generic error from middleware
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("Unauthorized - Expired Token", func(t *testing.T) {
		router, mockRepo, handler := setupTestRouterWithUserMocks()
		userGroup := router.Group("/users")
		userGroup.Use(authMiddleware)       
		userGroup.GET("/:id", handler.GetUserByID)
		// Generate token with negative expiration
		expiredToken, err := generateTestToken(authUserID, testJWTSecret, -5*time.Minute)
		assert.NoError(t, err)

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users/"+testID.String(), nil)
		request.Header.Set("Authorization", "Bearer "+expiredToken)
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Token has expired")
		mockRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})
}