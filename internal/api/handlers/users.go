// /home/bsant/testing/go-api-template/internal/api/handlers/users.go
package handlers

import (
	"errors" // Import errors for checking specific storage errors
	"log"
	"net/http"

	"go-api-template/internal/models"
	"go-api-template/internal/storage" // Use the interface package

	"github.com/gin-gonic/gin"
	"github.com/google/uuid" // Example for generating IDs if needed
)

// UserHandler holds the repository dependency for user operations
type UserHandler struct {
	repo storage.UserRepository
}

// NewUserHandler creates a new UserHandler with the given repository
func NewUserHandler(repo storage.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// GetUsers godoc
// @Summary      List all users
// @Description  Retrieves a list of all registered users.
// @Tags         users
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.User "Successfully retrieved list of users"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users [get]
func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.repo.GetAll(c.Request.Context()) // Use h.repo and pass context
	if err != nil {
		log.Printf("Error fetching users: %v", err) // Log the actual error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
		return
	}
	// No need to check for nil if GetAll guarantees an empty slice
	c.JSON(http.StatusOK, users)
}

// GetUserByID godoc
// @Summary      Get a user by ID
// @Description  Retrieves details for a specific user by their ID.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "User ID" Format(uuid) // Specify path param
// @Success      200  {object}  models.User "Successfully retrieved user"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [get]
func (h *UserHandler) GetUserByID(c *gin.Context) {
	id := c.Param("id") // Get ID from URL path

	user, err := h.repo.GetByID(c.Request.Context(), id) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else {
			log.Printf("Error fetching user by ID %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user"})
		}
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateUser godoc
// @Summary      Create a new user
// @Description  Adds a new user to the database. ID can be optionally provided or will be generated.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user body      models.User true  "User object to create" // Specify request body
// @Success      201  {object}  models.User "User created successfully"
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - User already exists"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var newUser models.User

	// Bind JSON request body to the newUser struct
	if err := c.ShouldBindJSON(&newUser); err != nil {
		log.Printf("Error binding user JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// --- ID Generation Strategy ---
	// If the client provides the ID, use newUser.ID directly.
	// If the server generates the ID (e.g., UUID):
	if newUser.ID.String() == "" { // Example: Generate UUID if ID is empty
		newUser.ID = uuid.New()
	}
	// If using SERIAL in DB, you'd modify repo.Create to return the ID

	// Validate required fields (example)
	if newUser.Name == "" || newUser.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and Email are required"})
		return
	}

	err := h.repo.Create(c.Request.Context(), &newUser) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "User with this ID or email already exists"})
		} else {
			log.Printf("Error creating user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		}
		return
	}

	// Return the created user (or just status Created)
	c.JSON(http.StatusCreated, newUser)
}

// UpdateUser godoc
// @Summary      Update an existing user
// @Description  Updates details for an existing user identified by ID.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id   path      string      true  "User ID" Format(uuid)
// @Param        user body      models.User true  "User object with updated fields"
// @Success      200  {object}  models.User "User updated successfully" // Or 204 No Content if not returning body
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      404  {object}  map[string]string{error=string} "User Not Found"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - e.g., duplicate email"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var userUpdates models.User

	if err := c.ShouldBindJSON(&userUpdates); err != nil {
		log.Printf("Error binding user update JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Basic validation
	if userUpdates.Name == "" || userUpdates.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and Email are required"})
		return
	}

	// Note: userUpdates.ID will be ignored by the repository method, which uses the 'id' from the URL.
	err := h.repo.Update(c.Request.Context(), id, &userUpdates) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		} else if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Update resulted in a conflict (e.g., duplicate email)"})
		} else {
			log.Printf("Error updating user %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		}
		return
	}

	// Optionally fetch and return the updated user, or just return No Content / OK
	// For simplicity, returning OK here. Fetching requires another DB call.
	// userUpdates.ID = id // Set the ID for the response if needed
	// c.JSON(http.StatusOK, userUpdates)
	c.Status(http.StatusOK) // Or http.StatusNoContent if nothing is returned
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

	err := h.repo.Delete(c.Request.Context(), id) // Use h.repo
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

