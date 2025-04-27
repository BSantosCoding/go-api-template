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

	// --- Register Resource Routes ---
	registerUserRoutes(apiV1, app.UserRepo)
	registerItemRoutes(apiV1, app.ItemRepo)

	// --- Health Check ---
	router.GET("/health", handlers.HealthCheck)

	// --- Swagger Route ---
	// REMOVED: swaggerURL construction

	log.Println("Configuring Swagger UI handler") // Optional log message

	// Register the Swagger UI handler WITHOUT the explicit URL option.
	// It will now load doc.json from a relative path (e.g., /swagger/doc.json)
	// and infer the host/port from the current browser location.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
