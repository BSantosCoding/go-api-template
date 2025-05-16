package handlers

import (
	"errors" // Import errors for checking specific storage errors
	"log"
	"net/http"

	"go-api-template/internal/api/middleware"
	"go-api-template/internal/services"
	"go-api-template/internal/storage" // Use the interface package
	"go-api-template/internal/transport/dto"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// UserHandler holds the repository dependency for user operations
type UserHandler struct {
	service   services.UserService // Use the service interface
	validator *validator.Validate
}

// NewUserHandler creates a new UserHandler with the given service
func NewUserHandler(userService services.UserService, validate *validator.Validate) *UserHandler {
	return &UserHandler{service: userService, validator: validate}
}

// GetUsers godoc
// @Summary      List all users
// @Description  Retrieves a list of all registered users.
// @Tags         users
// @Accept       json
// @Produce      json
// @Success      200  {array}   dto.UserResponse "Successfully retrieved list of users" // UPDATED response type
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users [get]
// @Security     BearerAuth
func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.service.GetAll(c.Request.Context()) // Use h.repo and pass context
	if err != nil {
		log.Printf("Error fetching users: %v", err) // Log the actual error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	userResponses := make([]dto.UserResponse, 0, len(users))
	for _, user := range users {
		// Need to pass a pointer if the helper expects one
		userResponses = append(userResponses, MapUserModelToUserResponse(user))
	}

	c.JSON(http.StatusOK, userResponses)
}

// GetUserByID godoc
// @Summary      Get a user by ID
// @Description  Retrieves details for a specific user by their ID.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "User ID" Format(uuid) // Specify path param
// @Success      200  {object}  dto.UserResponse "Successfully retrieved user" // Ensure this is already dto.UserResponse
// @Failure      400  {object}  map[string]string{error=string} "Invalid user ID format"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [get]
// @Security     BearerAuth
func (h *UserHandler) GetUserByID(c *gin.Context) {
	idStr := c.Param("id") // Get ID from URL path as string

	// Parse UUID and handle error
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	req := dto.GetUserByIdRequest{ID: parsedID}

	user, err := h.service.GetByID(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Error fetching user by ID %s: %v", idStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		}
		return
	}

	// Map to response DTO
	userResponse := MapUserModelToUserResponse(user) // Ensure mapping happens here too
	c.JSON(http.StatusOK, userResponse)
}

// --- Authentication Handlers ---

// Register godoc
// @Summary      Register a new user
// @Description  Adds a new user to the database with a hashed password.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user body      dto.CreateUserRequest true  "User registration details (ID is ignored/generated)"
// @Success      201  {object}  dto.UserResponse "User registered successfully"
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input or validation failed"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - Email already exists"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var req dto.CreateUserRequest

	// Bind JSON body
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Validate the request struct
	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	createdUser, err := h.service.Register(c.Request.Context(), &req) // Call storage Create
	if err != nil {
		// Check for specific duplicate email error
		if errors.Is(err, storage.ErrDuplicateEmail) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email address already registered"})
			// Check for general conflict (e.g., if ID was somehow duplicated, though unlikely now)
		} else if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "User conflict"})
		} else {
			log.Printf("Error registering user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		}
		return
	}

	// Map to response DTO to exclude sensitive info (like password hash)
	userResponse := MapUserModelToUserResponse(createdUser)

	c.JSON(http.StatusCreated, userResponse)
}

// Login godoc
// @Summary      Log in a user
// @Description  Authenticates a user based on email and password. Returns user details (and later a token).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body      dto.LoginRequest true  "User login credentials"
// @Success      200  {object}  dto.LoginResponse "Login successful"
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input or validation failed"
// @Failure      401  {object}  map[string]string{error=string} "Unauthorized - Invalid credentials"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	user, accessToken, refreshToken, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		} else {
			log.Printf("Error logging in user %s: %v", req.Email, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log in"})
		}
		return
	}

	userResponse := MapUserModelToUserResponse(user)
	loginResponse := dto.LoginResponse{
		User:         userResponse,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}

	log.Printf("User logged in successfully: %s", user.Email)
	c.JSON(http.StatusOK, loginResponse)
}

// Refresh godoc
// @Summary      Refresh access token
// @Description  Provides a new access token and refresh token using a valid refresh token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refreshRequest body      dto.RefreshRequest true  "Refresh token"
// @Success      200  {object}  dto.LoginResponse "Token refreshed successfully" // Reusing LoginResponse structure
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      401  {object}  map[string]string{error=string} "Unauthorized - Invalid or expired refresh token"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/refresh [post]
func (h *UserHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	newAccessToken, newRefreshToken, err := h.service.Refresh(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) { // Reuse error for invalid/expired refresh token
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		} else {
			log.Printf("Error refreshing token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		}
		return
	}

	refreshResponse := gin.H{
		"accessToken":  newAccessToken,
		"refreshToken": newRefreshToken,
	}

	log.Println("Token refreshed successfully")
	c.JSON(http.StatusOK, refreshResponse) // Consider a dedicated RefreshResponse DTO later
}

// Logout godoc
// @Summary      Log out user
// @Description  Invalidates the user's refresh token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        refreshRequest body      dto.RefreshRequest true  "Refresh token to invalidate" // Reusing RefreshRequest DTO
// @Success      204  {object}  nil "Logout successful"
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest // Reuse RefreshRequest to get the token

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if err := h.service.Logout(c.Request.Context(), &req); err != nil {
		log.Printf("Error during logout for token %s: %v", req.RefreshToken, err)
	}

	log.Printf("Logout successful for token: %s", req.RefreshToken)
	c.Status(http.StatusNoContent)
}

// UpdateUser godoc
// @Summary      Update an existing user
// @Description  Updates details for an existing user identified by ID.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      string      true  "User ID" Format(uuid)
// @Param        user body      dto.UpdateUserRequest true  "User object with updated fields" // Use DTO for body param
// @Success      200  {object}  dto.UserResponse "User updated successfully" // UPDATED response type
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input or validation failed"
// @Failure 	 401  {object}  map[string]string{error=string} "Unauthorized - Invalid token"
// @Failure      403  {object}  map[string]string{error=string} "Forbidden - Not allowed to update this user"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - e.g., duplicate email"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [put]
// @Security     BearerAuth
func (h *UserHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")

	// Parse UUID and handle error
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}
	req.ID = parsedID // Set ID from path

	if err := h.validator.Struct(req); err != nil {
		validationErrors := FormatValidationErrors(err.(validator.ValidationErrors))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": validationErrors})
		return
	}

	requestingUserId, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if requestingUserId != parsedID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not allowed to update this user"})
		return
	}

	updatedUser, err := h.service.Update(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Update resulted in a conflict"})
		} else {
			log.Printf("Error updating user %s: %v", idStr, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		}
		return
	}

	// Map to response DTO
	userResponse := MapUserModelToUserResponse(updatedUser) // UPDATED to map response
	c.JSON(http.StatusOK, userResponse)
}

// DeleteUser godoc
// @Summary      Delete a user by ID
// @Description  Removes a user from the database by their ID.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "User ID" Format(uuid)
// @Success      204  {object}  nil "User deleted successfully" // 204 No Content
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid user ID format"
// @Failure 	 401  {object}  map[string]string{error=string} "Unauthorized - Invalid token"
// @Failure      403  {object}  map[string]string{error=string} "Forbidden - Not allowed to delete this user"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [delete]
// @Security     BearerAuth
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	//Input validation
	var userDelete dto.DeleteUserRequest
	userDelete.ID = uuid.MustParse(id)

	if err := h.validator.Struct(userDelete); err != nil {
		// Handle validation errors
		validationErrors := err.(validator.ValidationErrors)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed", "details": FormatValidationErrors(validationErrors)})
		return
	}

	requestingUserId, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if requestingUserId != userDelete.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not allowed to delete this user"})
		return
	}

	err = h.service.Delete(c.Request.Context(), &userDelete) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Error deleting user %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		}
		return
	}

	c.Status(http.StatusNoContent) // Standard response for successful DELETE
}
