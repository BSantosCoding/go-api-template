package database

import (
	"database/sql"
	"fmt"
	"go-api-template/config"
	"go-api-template/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewEntClient(cfg config.DBConfig) (*ent.Client, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	entDriver := entsql.OpenDB(dialect.Postgres, db)
	entClient := ent.NewClient(ent.Driver(entDriver))

	return entClient, nil
}
