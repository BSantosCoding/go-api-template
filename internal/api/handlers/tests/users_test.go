package routes_test

import (
	"context"
	"encoding/json"
	"errors"
	"go-api-template/internal/api/handlers"
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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserHandler is a mock implementation of UserHandlerInterface
type MockUserHandler struct {
	mock.Mock
}

// Implement the interface methods for the mock
func (m *MockUserHandler) GetUserByID(c *gin.Context) {
	m.Called(c) // Record that the method was called
}

func (m *MockUserHandler) GetUsers(c *gin.Context) {
	m.Called(c)
}

func (m *MockUserHandler) CreateUser(c *gin.Context) {
	m.Called(c)
}

func (m *MockUserHandler) UpdateUser(c *gin.Context) {
	m.Called(c)
}

func (m *MockUserHandler) DeleteUser(c *gin.Context) {
	m.Called(c)
}

// Ensure MockUserHandler implements the interface (compile-time check)
var _ handlers.UserHandlerInterface = (*MockUserHandler)(nil)

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

func setupTestRouterWithUserMocks() (*gin.Engine, *MockUserRepository, *handlers.UserHandler) {
	gin.SetMode(gin.TestMode)
	mockRepo := new(MockUserRepository)
	validate := validator.New() // Use real validator
	handler := handlers.NewUserHandler(mockRepo, validate)
	router := gin.New()
	return router, mockRepo, handler
}

func TestRegisterUserRoutes(t *testing.T) {
	// Arrange
	gin.SetMode(gin.TestMode) // Set Gin to test mode

	mockHandler := new(MockUserHandler) // Create instance of the mock handler

	router := gin.New()              // Create a new Gin engine for testing
	testGroup := router.Group("/api") // Create a base group similar to potential real setup

	// Act
	routes.RegisterUserRoutes(testGroup, mockHandler) // Call the function under test

	// Assert
	// Check if the expected routes are registered
	expectedRoutes := []struct {
		Method string
		Path   string
	}{
		{http.MethodGet, "/api/users/:id"},
		{http.MethodGet, "/api/users/"},
		{http.MethodPost, "/api/users/"},
		{http.MethodPut, "/api/users/:id"},
		{http.MethodDelete, "/api/users/:id"},
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

func TestUserHandler_GetUsers(t *testing.T) {
	router, mockRepo, handler := setupTestRouterWithUserMocks()
	// If testing handlers directly, you don't need the router part here,
	// just call handler.GetUsers directly with a test context.
	// Assuming you are testing via HTTP requests as per previous examples:
	router.GET("/users", handler.GetUsers)

	t.Run("Success", func(t *testing.T) {
		// Arrange
		now := time.Now() // Use a fixed time for comparison if needed, or just compare instants
		expectedUsers := []models.User{
			{ID: uuid.New(), Name: "User 1", Email: "user1@example.com", CreatedAt: now, UpdatedAt: now},
			{ID: uuid.New(), Name: "User 2", Email: "user2@example.com", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}, // Example with different times
		}
		mockRepo.On("GetAll", mock.Anything).Return(expectedUsers, nil).Once()

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users", nil)
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)

		var responseUsers []models.User
		err := json.Unmarshal(recorder.Body.Bytes(), &responseUsers)
		assert.NoError(t, err)

		// Compare length first
		assert.Len(t, responseUsers, len(expectedUsers), "Number of users should match")

		// Compare elements individually
		for i := range expectedUsers {
			assert.Equal(t, expectedUsers[i].ID, responseUsers[i].ID, "User ID mismatch at index %d", i)
			assert.Equal(t, expectedUsers[i].Name, responseUsers[i].Name, "User Name mismatch at index %d", i)
			assert.Equal(t, expectedUsers[i].Email, responseUsers[i].Email, "User Email mismatch at index %d", i)

			// Compare time instants using time.Equal()
			assert.True(t, expectedUsers[i].CreatedAt.Equal(responseUsers[i].CreatedAt),
				"CreatedAt mismatch at index %d. Expected: %v, Got: %v", i, expectedUsers[i].CreatedAt, responseUsers[i].CreatedAt)
			assert.True(t, expectedUsers[i].UpdatedAt.Equal(responseUsers[i].UpdatedAt),
				"UpdatedAt mismatch at index %d. Expected: %v, Got: %v", i, expectedUsers[i].UpdatedAt, responseUsers[i].UpdatedAt)
		}
		// --------------------------

		mockRepo.AssertExpectations(t)
	})

	t.Run("Success - Empty List", func(t *testing.T) {
		// Arrange
		expectedUsers := []models.User{} // Empty slice
		mockRepo.On("GetAll", mock.Anything).Return(expectedUsers, nil).Once()

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users", nil)
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusOK, recorder.Code)

		var responseUsers []models.User
		err := json.Unmarshal(recorder.Body.Bytes(), &responseUsers)
		assert.NoError(t, err)
		// Direct comparison works for empty slices, but checking length is clearer
		assert.Len(t, responseUsers, 0)
		// assert.Equal(t, expectedUsers, responseUsers) // This is also fine for empty
		mockRepo.AssertExpectations(t)
	})

	t.Run("Internal Server Error", func(t *testing.T) {
		// Arrange
		mockRepo.On("GetAll", mock.Anything).Return(nil, errors.New("database error")).Once()

		// Act
		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest(http.MethodGet, "/users", nil)
		router.ServeHTTP(recorder, request)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "Failed to retrieve users")
		mockRepo.AssertExpectations(t)
	})
}