package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go-api-template/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewConnectionPool creates a new PostgreSQL connection pool using pgx.
func NewConnectionPool(cfg config.DBConfig) (*pgxpool.Pool, error) {
	// Example DSN: postgres://user:password@host:port/database?sslmode=disable
	// Adjust sslmode based on your environment (e.g., require, verify-full)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	// Configure pool settings (optional but recommended)
	poolConfig.MaxConns = 10 // Example: Set max connections
	poolConfig.MinConns = 2  // Example: Set min connections
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	// Health check interval ensures unhealthy connections are pruned
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	log.Println("Attempting to connect to database...")
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Ping the database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close() // Close the pool if ping fails
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection pool established successfully")
	return pool, nil
}
