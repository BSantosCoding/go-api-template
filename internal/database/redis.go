package database

import (
	"context"
	"fmt"
	"log"

	"go-api-template/config"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates and returns a new Redis client based on the provided configuration.
func NewRedisClient(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password, 
		DB:       cfg.DB,       
	})

	// Ping the Redis server to ensure connectivity
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", cfg.Addr, err)
	}
	log.Printf("Successfully connected to Redis at %s, DB %d", cfg.Addr, cfg.DB)
	return rdb, nil
}