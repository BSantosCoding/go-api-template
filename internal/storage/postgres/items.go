package postgres

import (
	"context"
	"errors"
	"log"

	"go-api-template/internal/models"
	"go-api-template/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ItemRepo implements the storage.ItemRepository interface using PostgreSQL.
type ItemRepo struct {
	db *pgxpool.Pool
}

// NewItemRepo creates a new ItemRepo.
func NewItemRepo(db *pgxpool.Pool) *ItemRepo {
	return &ItemRepo{db: db}
}

// Compile-time check to ensure ItemRepo implements ItemRepository
var _ storage.ItemRepository = (*ItemRepo)(nil)

func (r *ItemRepo) GetAll(ctx context.Context) ([]models.Item, error) {
	query := `SELECT id, name, description, price FROM items ORDER BY name ASC;`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		log.Printf("Error querying all items: %v\n", err)
		return nil, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Item])
	if err != nil {
		log.Printf("Error scanning items: %v\n", err)
		return nil, err
	}

	if items == nil {
		items = []models.Item{}
	}

	return items, nil
}

func (r *ItemRepo) GetByID(ctx context.Context, id string) (*models.Item, error) {
	query := `SELECT id, name, description, price FROM items WHERE id = $1;` 
	row := r.db.QueryRow(ctx, query, id)

	var item models.Item
	// Ensure the order matches the SELECT statement
	err := row.Scan(&item.ID, &item.Name, &item.Description, &item.Price)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		log.Printf("Error scanning item by ID %s: %v\n", id, err)
		return nil, err
	}
	return &item, nil
}

func (r *ItemRepo) Create(ctx context.Context, item *models.Item) error {
	// Assuming ID is provided
	query := `INSERT INTO items (id, name, description, price) VALUES ($1, $2, $3, $4);` 

	_, err := r.db.Exec(ctx, query, item.ID, item.Name, item.Description, item.Price)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Check unique constraint
			log.Printf("Attempted to create item with duplicate ID: %v\n", err)
			return storage.ErrConflict
		}
		log.Printf("Error creating item: %v\n", err)
		return err
	}
	return nil
}

func (r *ItemRepo) Update(ctx context.Context, id string, item *models.Item) error {
	query := `UPDATE items SET name = $1, description = $2, price = $3 WHERE id = $4;`

	cmdTag, err := r.db.Exec(ctx, query, item.Name, item.Description, item.Price, id)
	if err != nil {
		log.Printf("Error updating item %s: %v\n", id, err)
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}

	return nil
}

func (r *ItemRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM items WHERE id = $1;` 

	cmdTag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		log.Printf("Error deleting item %s: %v\n", id, err)
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return storage.ErrNotFound
	}

	return nil
}
