package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go-api-template/ent"
	"go-api-template/ent/job"
	"go-api-template/ent/predicate"

	"github.com/google/uuid"

	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto" // Import the DTO package
)

// JobRepo implements the storage.JobRepository interface using Ent.
type JobRepo struct {
	client *ent.Client
}

// NewJobRepo creates a new JobRepo.
func NewJobRepo(client *ent.Client) *JobRepo {
	return &JobRepo{client: client}
}

func (r *JobRepo) WithTx(tx *ent.Tx) storage.JobRepository {
	return &JobRepo{client: tx.Client()}
}

var _ storage.JobRepository = (*JobRepo)(nil)

// Create saves a new job posting using Ent.
// Returns the created *ent.Job entity.
func (r *JobRepo) Create(ctx context.Context, jobData *dto.CreateJobRequest) (*ent.Job, error) {
	createBuilder := r.client.Job.
		Create().
		SetRate(jobData.Rate).
		SetDuration(jobData.Duration).
		SetEmployerID(jobData.EmployerID).
		SetInvoiceInterval(jobData.InvoiceInterval)

	entJob, err := createBuilder.Save(ctx)
	if err != nil {
		// Check for constraint errors (like foreign key violations on employer_id)
		if ent.IsConstraintError(err) {
			log.Printf("Ent constraint error during job creation: %v", err)
			return nil, fmt.Errorf("failed to create job: constraint violation: %w", storage.ErrConflict)
		}
		log.Printf("Error creating job using Ent: %v\n", err)
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	log.Printf("Job created successfully with ID: %s", entJob.ID)
	return entJob, nil
}

// GetByID retrieves a specific job by its ID using Ent.
// Returns the found *ent.Job entity or storage.ErrNotFound if not found.
func (r *JobRepo) GetByID(ctx context.Context, params *dto.GetJobByIDRequest) (*ent.Job, error) {
	entJob, err := r.client.Job.Get(ctx, params.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job not found with ID: %s\n", params.ID)
			return nil, storage.ErrNotFound
		}
		log.Printf("Error getting job by ID %s using Ent: %v\n", params.ID, err)
		return nil, fmt.Errorf("failed to get job by ID %s: %w", params.ID, err)
	}

	return entJob, nil
}

// ListJobRepositoryParams is an internal struct to consolidate list parameters for the repository.
type ListJobRepositoryParams struct {
	Limit        int
	Offset       int
	MinRate      *float64
	MaxRate      *float64
	State        *job.State
	EmployerID   uuid.UUID
	ContractorID uuid.UUID
}

// buildJobQuery applies common filters, offset, and limit to an Ent query builder.
func (r *JobRepo) buildJobQuery(query *ent.JobQuery, params *ListJobRepositoryParams) *ent.JobQuery {
	predicates := []predicate.Job{}

	if params.MinRate != nil {
		predicates = append(predicates, job.RateGTE(*params.MinRate))
	}
	if params.MaxRate != nil {
		predicates = append(predicates, job.RateLTE(*params.MaxRate))
	}
	if params.State != nil {
		predicates = append(predicates, job.StateEQ(*params.State))
	}
	// EmployerID and ContractorID are handled in the specific list methods if they are mandatory filters.
	// They are included here in case buildJobQuery is reused for other list types where they are optional.
	if params.EmployerID != uuid.Nil {
		predicates = append(predicates, job.EmployerID(params.EmployerID))
	}
	if params.ContractorID != uuid.Nil {
		predicates = append(predicates, job.ContractorID(params.ContractorID))
	}

	if len(predicates) > 0 {
		query = query.Where(job.And(predicates...))
	}

	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}
	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	// Add ordering if needed, e.g., query = query.Order(ent.Asc(job.FieldCreatedAt))

	return query
}

// ListAvailable retrieves jobs that have no contractor assigned yet using Ent.
// Returns a slice of *ent.Job entities.
func (r *JobRepo) ListAvailable(ctx context.Context, params *dto.ListAvailableJobsRequest) ([]*ent.Job, error) {
	// Map DTO to internal repository params
	repoParams := &ListJobRepositoryParams{
		Limit:   params.Limit,
		Offset:  params.Offset,
		MinRate: params.MinRate,
		MaxRate: params.MaxRate,
		// State, EmployerID, ContractorID are not part of ListAvailableJobsRequest DTO
	}

	query := r.client.Job.
		Query().
		Where(
			job.ContractorIDIsNil(),
			job.StateEQ(job.StateWaiting), // Assume available jobs are in Waiting state
		)

	query = r.buildJobQuery(query, repoParams)

	entJobs, err := query.All(ctx)
	if err != nil {
		log.Printf("Error querying available jobs using Ent: %v\n", err)
		return nil, fmt.Errorf("failed to query available jobs: %w", err)
	}

	return entJobs, nil
}

// ListByEmployer retrieves jobs posted by a specific employer using Ent.
// Returns a slice of *ent.Job entities.
func (r *JobRepo) ListByEmployer(ctx context.Context, params *dto.ListJobsByEmployerRequest) ([]*ent.Job, error) {
	if params.EmployerID == uuid.Nil {
		return nil, errors.New("employer ID is required for listing jobs by employer")
	}

	// Map DTO to internal repository params
	repoParams := &ListJobRepositoryParams{
		Limit:      params.Limit,
		Offset:     params.Offset,
		MinRate:    params.MinRate,
		MaxRate:    params.MaxRate,
		State:      params.State,
		EmployerID: params.EmployerID, // Pass EmployerID to repoParams for buildJobQuery consistency
		// ContractorID is not part of ListJobsByEmployerRequest DTO
	}

	query := r.client.Job.
		Query().
		Where(job.EmployerID(params.EmployerID)) // Ensure the mandatory EmployerID filter is applied

	query = r.buildJobQuery(query, repoParams) // buildJobQuery will apply other optional filters (min/max rate, state)
	entJobs, err := query.All(ctx)
	if err != nil {
		log.Printf("Error querying jobs by employer %s using Ent: %v\n", params.EmployerID, err)
		return nil, fmt.Errorf("failed to query jobs by employer: %w", err)
	}

	return entJobs, nil
}

// ListByContractor retrieves jobs taken by a specific contractor using Ent.
// Returns a slice of *ent.Job entities.
func (r *JobRepo) ListByContractor(ctx context.Context, params *dto.ListJobsByContractorRequest) ([]*ent.Job, error) {
	if params.ContractorID == uuid.Nil {
		return nil, errors.New("contractor ID is required for listing jobs by contractor")
	}

	// Map DTO to internal repository params
	repoParams := &ListJobRepositoryParams{
		Limit:        params.Limit,
		Offset:       params.Offset,
		MinRate:      params.MinRate,
		MaxRate:      params.MaxRate,
		State:        params.State,
		ContractorID: params.ContractorID,
	}

	query := r.client.Job.
		Query().
		Where(job.ContractorID(params.ContractorID)) // Ensure the mandatory ContractorID filter is applied

	query = r.buildJobQuery(query, repoParams) // buildJobQuery will apply other optional filters (min/max rate, state)
	entJobs, err := query.All(ctx)
	if err != nil {
		log.Printf("Error querying jobs by contractor %s using Ent: %v\n", params.ContractorID, err)
		return nil, fmt.Errorf("failed to query jobs by contractor: %w", err)
	}

	return entJobs, nil
}

// Update modifies an existing job based on non-nil fields in the parameter struct using Ent.
// Returns the updated *ent.Job entity or storage.ErrNotFound if not found.
func (r *JobRepo) Update(ctx context.Context, params *dto.UpdateJobRequest) (*ent.Job, error) {
	updateBuilder := r.client.Job.UpdateOneID(params.ID)

	if params.Rate != nil {
		updateBuilder.SetRate(*params.Rate)
	}
	if params.Duration != nil {
		updateBuilder.SetDuration(*params.Duration)
	}
	// Handle ContractorID update: If the pointer is not nil, it means the field was
	// provided in the DTO. The value can be a real UUID or uuid.Nil (which in Ent
	// sets the foreign key to NULL). If the pointer is nil, the field was not provided,
	// and we make no change to the contractor_id column.
	if params.ContractorID != nil { // If the field is present in the DTO (not nil pointer)
		if *params.ContractorID == uuid.Nil {
			updateBuilder.ClearContractorID() // Explicitly clear if uuid.Nil is provided
		} else {
			updateBuilder.SetContractorID(*params.ContractorID) // Set to the provided UUID
		}
	}
	if params.State != nil {
		updateBuilder.SetState(*params.State)
	}

	entJob, err := updateBuilder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job not found for update with ID: %s\n", params.ID)
			return nil, storage.ErrNotFound
		}
		// Check for constraint violations during update (e.g., invalid contractor_id)
		if ent.IsConstraintError(err) {
			log.Printf("Ent constraint error during job update %s: %v", params.ID, err)
			return nil, fmt.Errorf("failed to update job %s: constraint violation: %w", params.ID, storage.ErrConflict)
		}
		log.Printf("Error updating job %s using Ent: %v\n", params.ID, err)
		return nil, fmt.Errorf("failed to update job %s: %w", params.ID, err)
	}

	log.Printf("Job updated successfully with ID: %s", entJob.ID)

	return entJob, nil
}

// Delete removes a job by its ID using Ent.
// Returns nil on success or storage.ErrNotFound if not found.
func (r *JobRepo) Delete(ctx context.Context, params *dto.DeleteJobRequest) error {
	err := r.client.Job.
		DeleteOneID(params.ID).
		Exec(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Job not found for delete with ID: %s\n", params.ID)
			return storage.ErrNotFound
		}
		// Check for constraint violations during delete (e.g., if the job is referenced by something else)
		if ent.IsConstraintError(err) {
			log.Printf("Ent constraint error during job delete %s: %v", params.ID, err)
			return fmt.Errorf("failed to delete job %s: constraint violation: %w", params.ID, storage.ErrConflict)
		}
		log.Printf("Error deleting job %s using Ent: %v\n", params.ID, err)
		return fmt.Errorf("failed to delete job %s: %w", params.ID, err)
	}

	log.Printf("Job deleted successfully with ID: %s", params.ID)

	return nil
}
