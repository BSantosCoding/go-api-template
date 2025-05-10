// /home/bsant/testing/go-api-template/main.go

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go-api-template/config"
	"go-api-template/internal/app"
	"go-api-template/internal/blockchain"
	"go-api-template/internal/database"
	"go-api-template/internal/server"

	_ "go-api-template/docs" // Import generated docs (will be created by swag init)

	"github.com/go-playground/validator"
)

// @title           Go API Template API
// @version         1.0
// @description     This is a sample server for a Go API template using Gin and pgx.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.example.com/support
// @contact.email  support@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1
// @schemes   http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// --- Initialize Redis Client ---
	redisClient, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	dbPool, err := database.NewConnectionPool(cfg.DB)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	// --- Initialize Blockchain Event Listener ---
	var eventListener *blockchain.EventListener
	if cfg.Blockchain.RPCURL != "" && cfg.Blockchain.ContractAddress != "" && cfg.Blockchain.ContractABIPath != "" {
		var err error
		eventListener, err = blockchain.NewEventListener(cfg.Blockchain.RPCURL, cfg.Blockchain.ContractAddress, cfg.Blockchain.ContractABIPath /*, pass services here */)
		if err != nil {
			log.Printf("WARN: Failed to initialize blockchain event listener: %v. Continuing without listener.", err)
		} else {
			eventListener.Start(context.Background()) // Start listening in the background
			log.Println("Blockchain event listener initialized and started")
		}
	} else {
		log.Println("Blockchain listener configuration missing (RPC URL, Address, or ABI Path), skipping initialization.")
	}

	validate := validator.New()

	application := &app.Application{
		Config:   cfg,
		DBPool:   dbPool,
		RedisClient: redisClient,
		Validator: validate,
	}

	srv := server.NewServer(application)

	// --- Graceful Shutdown Handling ---
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // Block until a signal is received

	log.Println("Shutting down server and listener...")

	// Stop the listener (if initialized)
	if eventListener != nil {
		eventListener.Stop()
	}

	//Gin shutdowns on its own

	log.Println("Application gracefully stopped.")
}

