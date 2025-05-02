package server

import (
	"fmt"
	"log"
	"time"

	"go-api-template/internal/api/routes"
	"go-api-template/internal/app"

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
		// AllowOrigins: app.Config.CORS.AllowedOrigins, // Use specific origins from config
		AllowOriginFunc: func(origin string) bool {
			// More flexible check: allow any origin in the list
			for _, allowed := range app.Config.CORS.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, // Common methods
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"}, // Common headers
		ExposeHeaders:    []string{"Content-Length"}, // Headers the browser is allowed to access
		AllowCredentials: true, // Allow cookies to be sent (if your frontend needs it)
		// AllowAllOrigins: true, // Alternative: Use this for very permissive CORS (less secure)
		MaxAge: 12 * time.Hour, // How long the result of a preflight request can be cached
	}
	router.Use(cors.New(corsConfig))
	// --- End CORS Configuration ---

	router.SetTrustedProxies(nil) // Remove the gin warning about untrusted proxies

	return &Server{
		router: router,
		app:    app,
	}
}

func (s *Server) Start() error {
	// Pass the container to routes
	routes.RegisterRoutes(s.router, s.app)

	addr := fmt.Sprintf("%s:%d", s.app.Config.Server.Host, s.app.Config.Server.Port) // Get config from container
	log.Printf("Server starting on %s", addr)
	return s.router.Run(addr)
}
