package database

import (
	"fmt"
	"go-api-template/config"
	"go-api-template/ent"
)

func NewEntClient(cfg config.DBConfig) (*ent.Client, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	entClient, err := ent.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open ent client: %w", err)
	}

	return entClient, nil
}
