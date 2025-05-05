// internal/storage/postgres/jobs.go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings" // For building SQL queries

	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // For checking specific errors
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobRepo implements the storage.JobRepository interface using PostgreSQL.
type JobRepo struct {
	db Querier
}

// NewJobRepo creates a new JobRepo.
func NewJobRepo(db *pgxpool.Pool) *JobRepo {
	return &JobRepo{db: db}
}
// WithTx creates a new JobRepo with the transaction.
func (r *JobRepo) WithTx(tx pgx.Tx) storage.JobRepository {
	return &JobRepo{db: tx}
}

// Compile-time check to ensure JobRepo implements JobRepository
var _ storage.JobRepository = (*JobRepo)(nil)

// Create saves a new job posting.
func (r *JobRepo) Create(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error) {
	job := &models.Job{
		ID:              uuid.New(), // Generate ID server-side
		Rate:            req.Rate,
		Duration:        req.Duration,
		EmployerID:      req.EmployerID, // Assumes EmployerID is set in the DTO by the handler
		State:           models.JobStateWaiting, // Default state
		InvoiceInterval: req.InvoiceInterval,
		// ContractorID is initially NULL
	}

	query := `
		INSERT INTO jobs (id, rate, duration, employer_id, state, invoice_interval, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, query,
		job.ID,
		job.Rate,
		job.Duration,
		job.EmployerID,
		job.State,
		job.InvoiceInterval,
	)

	var createdJob models.Job
	err := row.Scan(
		&createdJob.ID,
		&createdJob.Rate,
		&createdJob.Duration,
		&createdJob.ContractorID, // Will scan as NULL if not set
		&createdJob.EmployerID,
		&createdJob.State,
		&createdJob.InvoiceInterval,
		&createdJob.CreatedAt,
		&createdJob.UpdatedAt,
	)

	if err != nil {
		// Check for foreign key violation (e.g., employer_id doesn't exist)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			log.Printf("Error creating job: Foreign key violation (employer_id: %s): %v\n", req.EmployerID, err)
			return nil, fmt.Errorf("failed to create job: invalid employer ID: %w", storage.ErrConflict) // Or a more specific error
		}
		log.Printf("Error creating job: %v\n", err)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.Printf("Job created successfully with ID: %s", createdJob.ID)
	return &createdJob, nil
}

// GetByID retrieves a specific job by its ID.
func (r *JobRepo) GetByID(ctx context.Context, req *dto.GetJobByIDRequest) (*models.Job, error) {
	query := `
		SELECT id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
		FROM jobs
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, query, req.ID)

	var job models.Job
	err := row.Scan(
		&job.ID,
		&job.Rate,
		&job.Duration,
		&job.ContractorID,
		&job.EmployerID,
		&job.State,
		&job.InvoiceInterval,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job not found with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error scanning job by ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to get job by ID %s: %w", req.ID, err)
	}

	return &job, nil
}

// ListAvailable retrieves jobs that have no contractor assigned yet.
func (r *JobRepo) ListAvailable(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]models.Job, error) {
	baseQuery := `
		SELECT id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
		FROM jobs
	`
	conditions := []string{"contractor_id IS NULL", "state = $1"} // Base conditions for available jobs
	args := []interface{}{models.JobStateWaiting} // Start args with state

	// Add optional filters
	if req.MinRate != nil {
		args = append(args, *req.MinRate)
		conditions = append(conditions, fmt.Sprintf("rate >= $%d", len(args)))
	}
	if req.MaxRate != nil {
		args = append(args, *req.MaxRate)
		conditions = append(conditions, fmt.Sprintf("rate <= $%d", len(args)))
	}

	query := r.buildJobListQuery(baseQuery, conditions, &args, req.Offset, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying available jobs: %v\n", err)
		return nil, fmt.Errorf("failed to query available jobs: %w", err)
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Job])
	if err != nil {
		log.Printf("Error scanning available jobs: %v\n", err)
		return nil, fmt.Errorf("failed to scan available jobs: %w", err)
	}

	if jobs == nil {
		jobs = []models.Job{} // Return empty slice, not nil
	}

	return jobs, nil
}

// ListByEmployer retrieves jobs posted by a specific employer.
func (r *JobRepo) ListByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]models.Job, error) {
	baseQuery := `
		SELECT id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
		FROM jobs
	`
	conditions := []string{"employer_id = $1"}
	args := []interface{}{req.EmployerID}

	// Add optional filters
	if req.State != nil {
		args = append(args, *req.State)
		conditions = append(conditions, fmt.Sprintf("state = $%d", len(args)))
	}
	if req.MinRate != nil {
		args = append(args, *req.MinRate)
		conditions = append(conditions, fmt.Sprintf("rate >= $%d", len(args)))
	}
	if req.MaxRate != nil {
		args = append(args, *req.MaxRate)
		conditions = append(conditions, fmt.Sprintf("rate <= $%d", len(args)))
	}

	query := r.buildJobListQuery(baseQuery, conditions, &args, req.Offset, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying jobs by employer %s: %v\n", req.EmployerID, err)
		return nil, fmt.Errorf("failed to query jobs by employer: %w", err)
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Job])
	if err != nil {
		log.Printf("Error scanning jobs by employer %s: %v\n", req.EmployerID, err)
		return nil, fmt.Errorf("failed to scan jobs by employer: %w", err)
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	return jobs, nil
}

// ListByContractor retrieves jobs taken by a specific contractor.
func (r *JobRepo) ListByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]models.Job, error) {
	baseQuery := `
		SELECT id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
		FROM jobs
	`
	conditions := []string{"contractor_id = $1"}
	args := []interface{}{req.ContractorID}

	// Add optional filters
	if req.State != nil {
		args = append(args, *req.State)
		conditions = append(conditions, fmt.Sprintf("state = $%d", len(args)))
	}
	if req.MinRate != nil {
		args = append(args, *req.MinRate)
		conditions = append(conditions, fmt.Sprintf("rate >= $%d", len(args)))
	}
	if req.MaxRate != nil {
		args = append(args, *req.MaxRate)
		conditions = append(conditions, fmt.Sprintf("rate <= $%d", len(args)))
	}

	query := r.buildJobListQuery(baseQuery, conditions, &args, req.Offset, req.Limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying jobs by contractor %s: %v\n", req.ContractorID, err)
		return nil, fmt.Errorf("failed to query jobs by contractor: %w", err)
	}
	defer rows.Close()

	jobs, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Job])
	if err != nil {
		log.Printf("Error scanning jobs by contractor %s: %v\n", req.ContractorID, err)
		return nil, fmt.Errorf("failed to scan jobs by contractor: %w", err)
	}

	if jobs == nil {
		jobs = []models.Job{}
	}

	return jobs, nil
}

// Update modifies an existing job based on non-nil fields in the request DTO.
func (r *JobRepo) Update(ctx context.Context, req *dto.UpdateJobRequest) (*models.Job, error) {
	var setClauses []string
	args := []interface{}{}
	argID := 1

	// Build SET clauses dynamically
	if req.Rate != nil {
		args = append(args, *req.Rate)
		setClauses = append(setClauses, fmt.Sprintf("rate = $%d", argID))
		argID++
	}
	if req.Duration != nil {
		args = append(args, *req.Duration)
		setClauses = append(setClauses, fmt.Sprintf("duration = $%d", argID))
		argID++
	}
	if req.ContractorID != nil {
		args = append(args, *req.ContractorID)
		setClauses = append(setClauses, fmt.Sprintf("contractor_id = $%d", argID))
		argID++
	}
	if req.State != nil {
		args = append(args, *req.State)
		setClauses = append(setClauses, fmt.Sprintf("state = $%d", argID))
		argID++
	}

	if len(setClauses) == 0 {
		log.Printf("Update called for job %s with no fields to change.", req.ID)
		return nil, fmt.Errorf("no fields provided for update on job %s", req.ID)
	}

	// Add updated_at and WHERE clause
	setClauses = append(setClauses, "updated_at = NOW()")
	args = append(args, req.ID) // Add ID for WHERE clause

	query := fmt.Sprintf(`
		UPDATE jobs
		SET %s
		WHERE id = $%d
		RETURNING id, rate, duration, contractor_id, employer_id, state, invoice_interval, created_at, updated_at
	`, strings.Join(setClauses, ", "), argID)

	row := r.db.QueryRow(ctx, query, args...)

	var updatedJob models.Job
	err := row.Scan(
		&updatedJob.ID,
		&updatedJob.Rate,
		&updatedJob.Duration,
		&updatedJob.ContractorID,
		&updatedJob.EmployerID,
		&updatedJob.State,
		&updatedJob.InvoiceInterval,
		&updatedJob.CreatedAt,
		&updatedJob.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job not found for update with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		// Handle other potential errors (e.g., constraint violations if contractor_id is invalid)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			log.Printf("Error updating job %s: Foreign key violation: %v\n", req.ID, err)
			return nil, fmt.Errorf("failed to update job: invalid reference: %w", storage.ErrConflict)
		}
		log.Printf("Error updating job %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to update job %s: %w", req.ID, err)
	}

	log.Printf("Job updated successfully: %s", updatedJob.ID)
	return &updatedJob, nil
}

// Delete removes a job by its ID.
func (r *JobRepo) Delete(ctx context.Context, req *dto.DeleteJobRequest) error {
	query := `DELETE FROM jobs WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, req.ID)
	if err != nil {
		log.Printf("Error deleting job %s: %v\n", req.ID, err)
		return fmt.Errorf("failed to delete job %s: %w", req.ID, err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("Job not found for deletion with ID: %s\n", req.ID)
		return storage.ErrNotFound
	}

	log.Printf("Job deleted successfully: %s", req.ID)
	return nil
}

