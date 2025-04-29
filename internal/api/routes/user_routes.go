package routes

import (
	"go-api-template/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// registerUserRoutes registers all routes related to users
func RegisterUserRoutes(rg *gin.RouterGroup, userHandler handlers.UserHandlerInterface, authMiddleware gin.HandlerFunc) {
	// Define the sub-group for users (e.g., /api/v1/users)
	users := rg.Group("/users")
	users.Use(authMiddleware) // Apply JWT authentication middleware to all user routes
	{
		users.GET("/", userHandler.GetUsers)
		users.GET("/:id", userHandler.GetUserByID)
		users.PUT("/:id", userHandler.UpdateUser)
		users.DELETE("/:id", userHandler.DeleteUser)
	}

	// --- Authentication Routes ---
	// Create a sub-group for authentication (e.g., /api/v1/auth)
	auth := rg.Group("/auth")
	{
		auth.POST("/register", userHandler.Register) // Route for user registration
		auth.POST("/login", userHandler.Login)       // Route for user login
		// Add other auth routes here later (e.g., refresh token, password reset)
	}
}
