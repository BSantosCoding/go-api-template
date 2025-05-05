package postgres

import (
	"context"
	"errors"
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobApplicationRepo implements the storage.JobApplicationRepository interface using PostgreSQL.
type JobApplicationRepo struct {
	// Use Querier interface to allow both *pgxpool.Pool and pgx.Tx
	db Querier
}

// NewJobApplicationRepo creates a new JobApplicationRepo.
func NewJobApplicationRepo(db *pgxpool.Pool) *JobApplicationRepo {
	return &JobApplicationRepo{db: db}
}
// WithTx creates a new JobApplicationRepo with the transaction.
func (r *JobApplicationRepo) WithTx(tx pgx.Tx) storage.JobApplicationRepository {
	return &JobApplicationRepo{db: tx}
}
// Compile-time check to ensure JobApplicationRepo implements JobApplicationRepository
var _ storage.JobApplicationRepository = (*JobApplicationRepo)(nil)

func (r *JobApplicationRepo) Create(ctx context.Context, req *dto.CreateJobApplicationRequest) (*models.JobApplication, error) {
	jobApplication := &models.JobApplication{
		ID:              uuid.New(),
		ContractorID:     req.ContractorID,
		JobID:           req.JobID,
		State:           models.JobApplicationWaiting, 
	} // CreatedAt and UpdatedAt are set by the database

	query := `
		INSERT INTO job_application (id, contractor_id, job_id, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, contractor_id, job_id, state, created_at, updated_at
	`

	row := r.db.QueryRow(ctx, query,
		jobApplication.ID,
		jobApplication.ContractorID,
		jobApplication.JobID,
		jobApplication.State,
	)

	var createdJobApplication models.JobApplication
	err := row.Scan(
		&createdJobApplication.ID,
		&createdJobApplication.ContractorID, 
		&createdJobApplication.JobID,
		&createdJobApplication.State,
		&createdJobApplication.CreatedAt,
		&createdJobApplication.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" { // foreign_key_violation
				log.Printf("Error creating jobApplication: Foreign key violation (job_id: %s, contractor_id: %s): %v\n", req.JobID, req.ContractorID, err)
				return nil, fmt.Errorf("failed to create jobApplication: invalid job ID or contractor ID: %w", storage.ErrConflict)
			}
			if pgErr.Code == "23505" && pgErr.ConstraintName == "unique_application" { // unique_violation
				log.Printf("Error creating jobApplication: Unique constraint violation (job_id: %s, contractor_id: %s): %v\n", req.JobID, req.ContractorID, err)
				return nil, fmt.Errorf("failed to create jobApplication: application already exists: %w", storage.ErrConflict)
			}
		}
		log.Printf("Error creating jobApplication: %v\n", err)
		return nil, fmt.Errorf("failed to create jobApplication: %w", err)
	}

	log.Printf("Job application created successfully with ID: %s", createdJobApplication.ID)
	return &createdJobApplication, nil
}

func (r *JobApplicationRepo) GetByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*models.JobApplication, error) {
	query := `
		SELECT id, contractor_id, job_id, state, created_at, updated_at
		FROM job_application
		WHERE id = $1
	`

	row := r.db.QueryRow(ctx, query, req.ID)

	var jobApplication models.JobApplication
	err := row.Scan(
		&jobApplication.ID,
		&jobApplication.ContractorID,
		&jobApplication.JobID,
		&jobApplication.State,
		&jobApplication.CreatedAt,
		&jobApplication.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job application not found with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error scanning job application by ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to get job application by ID %s: %w", req.ID, err)
	}

	return &jobApplication, nil
}

func (r *JobApplicationRepo) ListByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]models.JobApplication, error) {
	var queryBuilder strings.Builder
	args := []interface{}{}
	argID := 1

	queryBuilder.WriteString(`
		SELECT id, contractor_id, job_id, state, created_at, updated_at
		FROM job_application
		WHERE contractor_id = $1 `)
	args = append(args, req.ContractorID)
	argID++

	queryBuilder.WriteString("ORDER BY created_at DESC")

	// Add LIMIT and OFFSET
	args = append(args, req.Limit)
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argID))
	argID++
	args = append(args, req.Offset)
	queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", argID))

	query := queryBuilder.String()

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying job applications by contractor ID %s: %v\n", req.ContractorID, err)
		return nil, fmt.Errorf("failed to list job applications by contractor: %w", err)
	}
	defer rows.Close()

	// Use pgx.CollectRows for potentially better performance and conciseness
	applications, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.JobApplication])
	if err != nil {
		log.Printf("Error scanning job application rows: %v\n", err)
		return nil, fmt.Errorf("failed to scan job applications: %w", err)
	}

	// Return empty slice instead of nil if no results
	if applications == nil {
		applications = []models.JobApplication{}
	}

	return applications, nil
}

func (r *JobApplicationRepo) ListByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]models.JobApplication, error) {
	var queryBuilder strings.Builder
	args := []interface{}{}
	argID := 1

	queryBuilder.WriteString(`
		SELECT id, contractor_id, job_id, state, created_at, updated_at
		FROM job_application
		WHERE job_id = $1 `)
	args = append(args, req.JobID)
	argID++

	queryBuilder.WriteString("ORDER BY created_at DESC")

	// Add LIMIT and OFFSET
	args = append(args, req.Limit)
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argID))
	argID++
	args = append(args, req.Offset)
	queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", argID))

	query := queryBuilder.String()

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("Error querying job applications by job ID %s: %v\n", req.JobID, err)
		return nil, fmt.Errorf("failed to list job applications by job: %w", err)
	}
	defer rows.Close()

	// Use pgx.CollectRows
	applications, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.JobApplication])
	if err != nil {
		log.Printf("Error scanning job application rows: %v\n", err)
		return nil, fmt.Errorf("failed to scan job applications: %w", err)
	}

	if applications == nil {
		applications = []models.JobApplication{}
	}

	return applications, nil
}

func (r *JobApplicationRepo) UpdateState(ctx context.Context, req *dto.UpdateJobApplicationStateRequest) (*models.JobApplication, error) {
	query := `
		UPDATE job_application
		SET state = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, contractor_id, job_id, state, created_at, updated_at
	`
	row := r.db.QueryRow(ctx, query, req.ID, req.State)

	var updatedApp models.JobApplication
	err := row.Scan(
		&updatedApp.ID,
		&updatedApp.ContractorID,
		&updatedApp.JobID,
		&updatedApp.State,
		&updatedApp.CreatedAt,
		&updatedApp.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job application not found for update with ID: %s\n", req.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error updating job application state for ID %s: %v\n", req.ID, err)
		return nil, fmt.Errorf("failed to update job application state: %w", err)
	}

	return &updatedApp, nil
}

// UpdateStateByJobID updates the state of all applications for a specific job.
// Useful for rejecting other applications when one is accepted.
func (r *JobApplicationRepo) UpdateStateByJobID(ctx context.Context, jobID uuid.UUID, newState models.JobApplicationState, excludeApplicationID *uuid.UUID) error {
	query := `
		UPDATE job_application
		SET state = $1, updated_at = NOW()
		WHERE job_id = $2 AND state = $3`
	args := []interface{}{newState, jobID, models.JobApplicationWaiting} // Only update 'Waiting' applications

	if excludeApplicationID != nil {
		query += " AND id != $4"
		args = append(args, *excludeApplicationID)
	}

	cmdTag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		log.Printf("Error updating states for job applications of job %s: %v\n", jobID, err)
		return fmt.Errorf("failed to update job application states for job %s: %w", jobID, err)
	}

	log.Printf("Updated %d job applications for job %s to state %s", cmdTag.RowsAffected(), jobID, newState)
	return nil
}

func (r *JobApplicationRepo) Delete(ctx context.Context, req *dto.DeleteJobApplicationRequest) error {
	query := `DELETE FROM job_application WHERE id = $1`

	cmdTag, err := r.db.Exec(ctx, query, req.ID)
	if err != nil {
		log.Printf("Error deleting job application with ID %s: %v\n", req.ID, err)
		return fmt.Errorf("failed to delete job application: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		log.Printf("Job application not found for deletion with ID: %s\n", req.ID)
		return storage.ErrNotFound
	}

	log.Printf("Job application deleted successfully with ID: %s", req.ID)
	return nil
}