// /home/bsant/testing/go-api-template/internal/api/routes/routes.go
package routes

import (
	// "fmt" // No longer needed here
	"go-api-template/internal/api/handlers"
	"go-api-template/internal/app"
	"log" // Keep log if you want the startup message

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterRoutes sets up the API routes by calling resource-specific registration functions
func RegisterRoutes(router *gin.Engine, app *app.Application) {

	// --- Base API Group ---
	apiV1 := router.Group("/api/v1")

	//Create handlers
	userHandler := handlers.NewUserHandler(app.UserRepo, app.Validator)
	itemHandler := handlers.NewItemHandler(app.ItemRepo, app.Validator)


	// --- Register Resource Routes ---
	RegisterUserRoutes(apiV1, userHandler)
	RegisterItemRoutes(apiV1, itemHandler)

	// --- Health Check ---
	router.GET("/health", handlers.HealthCheck)

	log.Println("Configuring Swagger UI handler") 
	// Register the Swagger UI handler WITHOUT the explicit URL option.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
