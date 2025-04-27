// /home/bsant/testing/go-api-template/main.go

package main

import (
	"log"

	"go-api-template/config"
	"go-api-template/internal/app"
	"go-api-template/internal/database"
	"go-api-template/internal/server"
	"go-api-template/internal/storage/postgres"

	_ "go-api-template/docs" // Import generated docs (will be created by swag init)
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
// @schemes   http https      // Schemes your API supports

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dbPool, err := database.NewConnectionPool(cfg.DB)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	userRepo := postgres.NewUserRepo(dbPool)
	itemRepo := postgres.NewItemRepo(dbPool)

	application := &app.Application{
		Config:   cfg,
		DBPool:   dbPool,
		UserRepo: userRepo,
		ItemRepo: itemRepo,
	}

	srv := server.NewServer(application)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

