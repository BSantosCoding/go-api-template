package routes

import (
	"go-api-template/internal/api/handlers"

	"github.com/gin-gonic/gin"
)

// registerUserRoutes registers all routes related to users
func RegisterUserRoutes(rg *gin.RouterGroup, userHandler handlers.UserHandlerInterface) {
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
