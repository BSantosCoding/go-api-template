// /home/bsant/testing/go-api-template/internal/api/routes/routes.go
package routes

import (
	// "fmt" // No longer needed here
	"go-api-template/internal/api/handlers"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/app"
	"go-api-template/internal/services"
	"log"

	"github.com/gin-gonic/gin"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// RegisterRoutes sets up the API routes by calling resource-specific registration functions
func RegisterRoutes(router *gin.Engine, app *app.Application) {

	// --- Base API Group ---
	apiV1 := router.Group("/api/v1")

	//Create services
	userService := services.NewUserService(app.UserRepo, app.RedisClient,app.Config.JWT.Secret, app.Config.JWT.Expiration, app.Config.JWT.RefreshExpiration)
	jobService := services.NewJobService(app.JobRepo, app.UserRepo)
	invoiceService := services.NewInvoiceService(app.InvoiceRepo, app.JobRepo)

	//Create handlers
	userHandler := handlers.NewUserHandler(userService, app.Validator)
	jobHandler := handlers.NewJobHandler(jobService, app.Validator)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceService, app.Validator)

	// --- Middleware ---
	authMiddleware := middleware.JWTAuthMiddleware(app.Config.JWT.Secret)

	// --- Register Resource Routes ---
	RegisterUserRoutes(apiV1, userHandler, authMiddleware)
	RegisterInvoiceRoutes(apiV1, invoiceHandler, authMiddleware)
	RegisterJobRoutes(apiV1, jobHandler, authMiddleware)

	// --- Health Check ---
	router.GET("/health", handlers.HealthCheck)

	log.Println("Configuring Swagger UI handler") 
	// Register the Swagger UI handler WITHOUT the explicit URL option.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
