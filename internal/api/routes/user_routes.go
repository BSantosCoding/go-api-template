package routes

import (
	"go-api-template/internal/api/handlers"
	"go-api-template/internal/storage" // Need storage interface

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
)

// registerUserRoutes registers all routes related to users
func registerUserRoutes(rg *gin.RouterGroup, userRepo storage.UserRepository, validate *validator.Validate) {
	// Create the handler specific to this resource
	userHandler := handlers.NewUserHandler(userRepo, validate)

	// Define the sub-group for users (e.g., /api/v1/users)
	users := rg.Group("/users")
	{
		users.GET("/", userHandler.GetUsers)
		users.POST("/", userHandler.CreateUser)
		users.GET("/:id", userHandler.GetUserByID)
		users.PUT("/:id", userHandler.UpdateUser)
		users.DELETE("/:id", userHandler.DeleteUser)
	}
}
