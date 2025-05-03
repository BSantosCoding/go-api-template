// internal/app/app.go (or similar package)
package app

import (
	"go-api-template/config"
	"go-api-template/internal/storage"

	"github.com/go-playground/validator"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Application holds core application dependencies.
type Application struct {
	Config   *config.Config
	DBPool   *pgxpool.Pool
	RedisClient *redis.Client
	UserRepo storage.UserRepository
	JobRepo storage.JobRepository
	InvoiceRepo storage.InvoiceRepository
	Validator *validator.Validate
}
