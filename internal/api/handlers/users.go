package handlers

import (
	"errors" // Import errors for checking specific storage errors
	"log"
	"net/http"

	"go-api-template/internal/storage" // Use the interface package
	"go-api-template/internal/transport/dto"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserHandler holds the repository dependency for user operations
type UserHandler struct {
	repo storage.UserRepository
	validator *validator.Validate
}

// NewUserHandler creates a new UserHandler with the given repository
func NewUserHandler(repo storage.UserRepository, validate *validator.Validate) *UserHandler {
	return &UserHandler{repo: repo, validator: validate}
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
func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.repo.GetAll(c.Request.Context()) // Use h.repo and pass context
	if err != nil {
		log.Printf("Error fetching users: %v", err) // Log the actual error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}

	userResponses := make([]dto.UserResponse, 0, len(users))
	for _, user := range users {
		// Need to pass a pointer if the helper expects one
		userResponses = append(userResponses, MapUserModelToUserResponse(&user))
	}

	c.JSON(http.StatusOK, userResponses) // UPDATED to return mapped slice
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
func (h *UserHandler) GetUserByID(c *gin.Context) {
	idStr := c.Param("id") // Get ID from URL path as string

	// Parse UUID and handle error
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	req := dto.GetUserByIdRequest{ID: parsedID}

	user, err := h.repo.GetByID(c.Request.Context(), &req)
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
// @Failure      400  {object}  map[string]interface{}{error=string, details=map[string]string} "Bad Request - Invalid input or validation failed"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - Email already exists"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/register [post] // Changed route example
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

	createdUser, err := h.repo.Create(c.Request.Context(), &req) // Call storage Create
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
// @Failure      400  {object}  map[string]interface{}{error=string, details=map[string]string} "Bad Request - Invalid input or validation failed"
// @Failure      401  {object}  map[string]string{error=string} "Unauthorized - Invalid credentials"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /auth/login [post] // Changed route example
func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

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

	// Prepare DTO for GetByEmail storage call
	emailReq := dto.GetUserByEmailRequest{Email: req.Email}

	// Fetch user by email - storage layer MUST return the password hash here
	user, err := h.repo.GetByEmail(c.Request.Context(), &emailReq)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// IMPORTANT: Return generic unauthorized error for both not found and bad password
			log.Printf("Login attempt failed for email %s: user not found", req.Email)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		} else {
			log.Printf("Error fetching user by email %s during login: %v", req.Email, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Login failed"})
		}
		return
	}

	// Compare the provided password with the stored hash
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		// Passwords don't match (bcrypt.ErrMismatchedHashAndPassword) or other error
		log.Printf("Login attempt failed for email %s: invalid password", req.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// --- Login Successful ---

	// Map to response DTO
	userResponse := MapUserModelToUserResponse(user)

	// Prepare login response (token will be added later)
	loginResponse := dto.LoginResponse{
		User: userResponse,
		// Token: "generate_jwt_here", // Placeholder for JWT generation
	}

	log.Printf("User logged in successfully: %s", user.Email)
	c.JSON(http.StatusOK, loginResponse)
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
// @Failure      400  {object}  map[string]interface{}{error=string, details=map[string]string} "Bad Request - Invalid input"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - e.g., duplicate email"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [put]
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

	updatedUser, err := h.repo.Update(c.Request.Context(), &req)
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
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [delete]
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

	err := h.repo.Delete(c.Request.Context(), &userDelete) // Use h.repo
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

