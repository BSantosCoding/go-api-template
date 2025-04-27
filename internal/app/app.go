// internal/app/app.go (or similar package)
package app

import (
	"go-api-template/config"
	"go-api-template/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Application holds core application dependencies.
type Application struct {
	Config   *config.Config
	DBPool   *pgxpool.Pool
	UserRepo storage.UserRepository
	ItemRepo storage.ItemRepository
	// Add other repositories 
	// Add services maybe
}
