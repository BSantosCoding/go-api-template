package server

import (
	"fmt"
	"log"
	"time"

	"go-api-template/internal/api"
	"go-api-template/internal/api/middleware" // Ensure middleware is imported
	"go-api-template/internal/app"
	"go-api-template/internal/services"

	oapi_middleware "github.com/oapi-codegen/gin-middleware"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
	app    *app.Application // Store the application container
}

func NewServer(app *app.Application) *Server {
	router := gin.Default()

	// --- Configure and Apply CORS Middleware ---
	log.Printf("Configuring CORS for origins: %v", app.Config.CORS.AllowedOrigins)
	corsConfig := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, allowed := range app.Config.CORS.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))
	// --- End CORS Configuration ---

	router.SetTrustedProxies(nil) // Remove the gin warning about untrusted proxies

	spec, err := api.GetSwagger()
	if err != nil {
		log.Fatalf("Error loading swagger spec: %v", err) // Use log.Fatalf as this is a critical error
		return nil                                        // Return nil in case of error
	}

	// Create the oapi-codegen validator middleware
	validator := oapi_middleware.OapiRequestValidatorWithOptions(spec,
		&oapi_middleware.Options{ // Use oapi_middleware.Options
			Options: openapi3filter.Options{
				AuthenticationFunc: middleware.JWTAuthenticationFunc(app.Config.JWT.Secret),
			},
		},
	)

	router.Use(validator)

	serverDefinition := services.NewServerDefinition(
		app.EntClient,
		app.RedisClient,
		app.Config.JWT.Secret,
		app.Config.JWT.Expiration,
		app.Config.JWT.RefreshExpiration,
	)

	strictHandler := api.NewStrictHandler(serverDefinition, []api.StrictMiddlewareFunc{})

	api.RegisterHandlers(router, strictHandler)

	// Apply the validator middleware to the router
	router.Use(validator)

	return &Server{
		router: router,
		app:    app,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.app.Config.Server.Host, s.app.Config.Server.Port) // Get config from container
	log.Printf("Server starting on %s", addr)
	return s.router.Run(addr)
}
