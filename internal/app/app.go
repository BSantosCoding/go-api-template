// internal/app/app.go (or similar package)
package app

import (
	"go-api-template/config"
	"go-api-template/ent"

	"github.com/redis/go-redis/v9"
)

// Application holds core application dependencies.
type Application struct {
	Config      *config.Config
	EntClient   *ent.Client
	RedisClient *redis.Client
}
