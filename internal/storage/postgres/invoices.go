// internal/storage/postgres/invoices.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings" // For building SQL query

	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // For checking specific errors
	"github.com/jackc/pgx/v5/pgxpool"
)

// InvoiceRepo implements the storage.InvoiceRepository interface using PostgreSQL.
type InvoiceRepo struct {
	db *pgxpool.Pool
}

// NewInvoiceRepo creates a new InvoiceRepo.
func NewInvoiceRepo(db *pgxpool.Pool) *InvoiceRepo {
	return &InvoiceRepo{db: db}
}

// Compile-time check to ensure InvoiceRepo implements InvoiceRepository
var _ storage.InvoiceRepository = (*InvoiceRepo)(nil)

// Create saves a new invoice for a job.
func (r *InvoiceRepo) Create(ctx context.Context, invoice *models.Invoice) (*models.Invoice, error) { 
	if invoice.ID == uuid.Nil {
		invoice.ID = uuid.New()
	}
	if invoice.State == "" {
		invoice.State = models.InvoiceStateWaiting // Ensure default if not set
	}

	// Insert the Invoice using data from the input model
	query := `
		INSERT INTO invoices (id, value, state, job_id, interval_number, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, value, state, job_id, interval_number, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query,
		invoice.ID,
		invoice.Value,          // Use value from input model
		invoice.State,          // Use state from input model
		invoice.JobID,
		invoice.IntervalNumber, // Use interval number from input model
	)

	var createdInvoice models.Invoice
	err := row.Scan(
		&createdInvoice.ID,
		&createdInvoice.Value,
		&createdInvoice.State,
		&createdInvoice.JobID,
		&createdInvoice.IntervalNumber,
		&createdInvoice.CreatedAt,
		&createdInvoice.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503": // foreign_key_violation (job_id doesn't exist)
				log.Printf("Error creating invoice: Foreign key violation (job_id: %s): %v\n", invoice.JobID, err)
				return nil, fmt.Errorf("failed to create invoice: invalid job ID: %w", storage.ErrConflict)
			case "23505": // unique_violation (job_id + interval_number)
				log.Printf("Error creating invoice: Unique constraint violation (job_id: %s, interval: %d): %v\n", invoice.JobID, invoice.IntervalNumber, err)
				return nil, fmt.Errorf("failed to create invoice: invoice for interval %d already exists: %w", invoice.IntervalNumber, storage.ErrConflict)
			}
		}
		log.Printf("Error creating invoice: %v\n", err)
		return nil, fmt.Errorf("failed to save invoice: %w", err)
	}

	log.Printf("Invoice created successfully with ID: %s for Job ID: %s", createdInvoice.ID, createdInvoice.JobID)
	return &createdInvoice, nil
}

// GetByID retrieves a specific invoice by its ID.
func (r *InvoiceRepo) GetByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*models.Invoice, error) {
	query := `
		SELECT id, value, state, job_id, interval_number, created_at, updated_at
		FROM invoices
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, req.ID)

	var invoice models.Invoice
	err := row.Scan(
		&invoice.ID,
		&invoice.Value,
		&invoice.State,
		&invoice.JobID,
		&invoice.IntervalNumber,
		&invoice.CreatedAt,
		&invoice.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Invoice not found with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error scanning invoice by ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to get invoice by ID %s: %w", req.ID, err)
	}

	return &invoice, nil
}

// ListByJob retrieves all invoices associated with a specific job, with optional state filtering.
func (r *InvoiceRepo) ListByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]models.Invoice, error) {
	var queryBuilder strings.Builder
	args := []interface{}{}
	argID := 1

	queryBuilder.WriteString(`
		SELECT id, value, state, job_id, interval_number, created_at, updated_at
		FROM invoices
		WHERE job_id = $1
	`)
	args = append(args, req.JobID)
	argID++

	// Add optional state filter
	if req.State != nil {
		queryBuilder.WriteString(fmt.Sprintf(" AND state = $%d", argID))
		args = append(args, *req.State)
		argID++
	}

	queryBuilder.WriteString(" ORDER BY interval_number ASC") // Order by interval

	// Add LIMIT and OFFSET
	args = append(args, req.Limit)
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argID))
	argID++
	args = append(args, req.Offset)
	queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", argID))

	query := queryBuilder.String()

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying invoices by job %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to query invoices by job: %w", err)
	}
	defer rows.Close()

	invoices, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Invoice])
	if err != nil {
		log.Printf("Error scanning invoices by job %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to scan invoices by job: %w", err)
	}

	if invoices == nil {
		invoices = []models.Invoice{} // Return empty slice, not nil
	}

	return invoices, nil
}

// UpdateState modifies the state of an existing invoice.
func (r *InvoiceRepo) UpdateState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*models.Invoice, error) {
	query := `
		UPDATE invoices
		SET state = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, value, state, job_id, interval_number, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query, req.NewState, req.ID)

	var updatedInvoice models.Invoice
	err := row.Scan(
		&updatedInvoice.ID,
		&updatedInvoice.Value,
		&updatedInvoice.State,
		&updatedInvoice.JobID,
		&updatedInvoice.IntervalNumber,
		&updatedInvoice.CreatedAt,
		&updatedInvoice.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Invoice not found for state update with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error updating invoice state %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to update invoice state %s: %w", req.ID, err)
	}

	log.Printf("Invoice state updated successfully for ID: %s to %s", updatedInvoice.ID, updatedInvoice.State)
	return &updatedInvoice, nil
}

// Delete removes an invoice by its ID.
func (r *InvoiceRepo) Delete(ctx context.Context, req *dto.DeleteInvoiceRequest) error {
	query := `DELETE FROM invoices WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, req.ID)
	if err != nil {
		log.Printf("Error deleting invoice %s: %v\n", req.ID, err)
		return fmt.Errorf("failed to delete invoice %s: %w", req.ID, err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("Invoice not found for deletion with ID: %s\n", req.ID)
		return storage.ErrNotFound
	}

	log.Printf("Invoice deleted successfully: %s", req.ID)
	return nil
}

// GetMaxIntervalForJob retrieves the highest interval number for a given job.
func (r *InvoiceRepo) GetMaxIntervalForJob(ctx context.Context, req *dto.GetMaxIntervalForJobRequest) (int, error) { // CHANGED: Accepts DTO
	var maxInterval sql.NullInt32
	query := `SELECT MAX(interval_number) FROM invoices WHERE job_id = $1`

	// Use req.JobID from the DTO
	err := r.db.QueryRow(ctx, query, req.JobID).Scan(&maxInterval)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // No invoices yet, max interval is 0
		}
		log.Printf("Error querying max interval for job %s: %v\n", req.JobID, err) // Use req.JobID in log
		return 0, fmt.Errorf("failed to query max interval number for job %s: %w", req.JobID, err) // Use req.JobID in error
	}

	if maxInterval.Valid {
		return int(maxInterval.Int32), nil
	}

	return 0, nil // Should be covered by ErrNoRows, but return 0 as default
}

