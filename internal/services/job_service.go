package services

import (
	"context"
	"fmt"
	"log"

	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"

	"github.com/jackc/pgx/v5/pgxpool"
)

type jobService struct {
	jobRepo storage.JobRepository
	userRepo storage.UserRepository
	db      *pgxpool.Pool // Add DB pool for transactions
}

// NewJobService creates a new instance of JobService.
func NewJobService(db *pgxpool.Pool) JobService {
	return &jobService{jobRepo: postgres.NewJobRepo(db), userRepo: postgres.NewUserRepo(db), db: db}
}

func (s *jobService) CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error) {
	// EmployerID is already set in the handler from context, passed in req.
	// Add validation if needed (e.g., check if EmployerID exists in user table)
	job, err := s.jobRepo.Create(ctx, req)
	if err != nil {
		log.Printf("JobService: Error creating job: %v", err)
		// Map storage errors if necessary (e.g., ErrConflict for FK violation)
		return nil, fmt.Errorf("internal error creating job: %w", err)
	}
	return job, nil
}

func (s *jobService) GetJobByID(ctx context.Context, req *dto.GetJobByIDRequest) (*models.Job, error) {
	job, err := s.jobRepo.GetByID(ctx, req)
	if err != nil {
		log.Printf("JobService: Error getting job %s: %v", req.ID, err)
		return nil, mapRepoError(err, "getting job by ID")
	}
	return job, nil
}

func (s *jobService) ListAvailableJobs(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]models.Job, error) {
	jobs, err := s.jobRepo.ListAvailable(ctx, req)
	if err != nil {
		log.Printf("JobService: Error listing available jobs: %v", err)
		return nil, fmt.Errorf("internal error listing available jobs: %w", err)
	}
	return jobs, nil
}

func (s *jobService) ListJobsByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]models.Job, error) {
	// EmployerID is set in handler from context and passed in req. (Might change this so it can be overridden to allow listing for other users)
	jobs, err := s.jobRepo.ListByEmployer(ctx, req)
	if err != nil {
		log.Printf("JobService: Error listing employer jobs for %s: %v", req.EmployerID, err)
		return nil, fmt.Errorf("internal error listing employer jobs: %w", err)
	}
	return jobs, nil
}

func (s *jobService) ListJobsByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]models.Job, error) {
	// ContractorID is set in handler from context and passed in req. (Might change this so it can be overridden to allow listing for other users)
	jobs, err := s.jobRepo.ListByContractor(ctx, req)
	if err != nil {
		log.Printf("JobService: Error listing contractor jobs for %s: %v", req.ContractorID, err)
		return nil, fmt.Errorf("internal error listing contractor jobs: %w", err)
	}
	return jobs, nil
}

func (s *jobService) UpdateJobDetails(ctx context.Context, req *dto.UpdateJobDetailsRequest) (*models.Job, error) {
	// --- Transaction Start ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("UpdateJobDetails: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if anything fails

	// Use transaction-aware repository
	txJobRepo := s.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := txJobRepo.GetByID(ctx, &getReq) // Use txJobRepo
	if err != nil {
		log.Printf("UpdateJobDetails: Error fetching job %s: %v", req.JobID, err)
		return nil, mapRepoError(err, "fetching job for update")
	}

	// Authorization & State Check
	if !(req.UserID == existingJob.EmployerID && existingJob.State == models.JobStateWaiting && existingJob.ContractorID == nil) {
		log.Printf("UpdateJobDetails: Forbidden attempt on job %s by user %s. State: %s, Contractor: %v", req.JobID, req.UserID, existingJob.State, existingJob.ContractorID)
		return nil, ErrForbidden // Or ErrInvalidState
	}

	updateRepoReq := dto.UpdateJobRequest{
		ID:       req.JobID,
		Rate:     req.Rate,
		Duration: req.Duration,
	}
	updatedJob, err := txJobRepo.Update(ctx, &updateRepoReq) // Use txJobRepo
	if err != nil {
		log.Printf("UpdateJobDetails: Error updating job %s in repo: %v", req.JobID, err)
		return nil, mapRepoError(err, "updating job details")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("UpdateJobDetails: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---
	return updatedJob, nil
}

func (s *jobService) UpdateJobState(ctx context.Context, req *dto.UpdateJobStateRequest) (*models.Job, error) {
	// --- Transaction Start ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("UpdateJobState: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if anything fails

	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := s.jobRepo.WithTx(tx).GetByID(ctx, &getReq) // Use tx repo
	if err != nil {
		log.Printf("UpdateJobState: Error fetching job %s: %v", req.JobID, err)
		return nil, mapRepoError(err, "fetching job for state update")
	}

	// Authorization check
	isEmployer := existingJob.EmployerID == req.UserID
	isCurrentContractor := existingJob.ContractorID != nil && *existingJob.ContractorID == req.UserID
	if !(isEmployer || isCurrentContractor) {
		log.Printf("UpdateJobState: Forbidden attempt on job %s by user %s. Role: Employer=%t, Contractor=%t", req.JobID, req.UserID, isEmployer, isCurrentContractor)
		return nil, ErrForbidden
	}

	// Prevent manual state change to Ongoing - this should only happen via AcceptApplication
	if req.State == models.JobStateOngoing && existingJob.State == models.JobStateWaiting {
		log.Printf("UpdateJobState: Forbidden attempt to manually set job %s to Ongoing by user %s.", req.JobID, req.UserID)
		return nil, fmt.Errorf("%w: cannot manually set state to Ongoing, use AcceptApplication", ErrInvalidTransition)
	}

	// Validation: Check state transition
	if !isValidJobStateTransition(existingJob.State, req.State) {
		return nil, fmt.Errorf("%w: from %s to %s", ErrInvalidTransition, existingJob.State, req.State)
	}

	newState := req.State
	updateRepoReq := dto.UpdateJobRequest{
		ID:    req.JobID,
		State: &newState,
	}
	updatedJob, err := s.jobRepo.WithTx(tx).Update(ctx, &updateRepoReq) // Use tx repo
	if err != nil {
		log.Printf("UpdateJobState: Error updating job state %s in repo: %v", req.JobID, err)
		return nil, mapRepoError(err, "updating job state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("UpdateJobState: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---
	return updatedJob, nil
}

func (s *jobService) DeleteJob(ctx context.Context, req *dto.DeleteJobRequest) error {
	// --- Transaction Start ---
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("DeleteJob: Error beginning transaction: %v", err)
		return fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if anything fails

	getReq := dto.GetJobByIDRequest{ID: req.ID}
	existingJob, err := s.jobRepo.WithTx(tx).GetByID(ctx, &getReq) // Use tx repo
	if err != nil {
		log.Printf("DeleteJob: Error fetching job %s for delete check: %v", req.ID, err)
		return mapRepoError(err, "fetching job for delete check")
	}

	// Authorization Check
	if existingJob.EmployerID != req.UserID {
		log.Printf("DeleteJob: Forbidden attempt on job %s by non-employer user %s", req.ID, req.UserID)
		return ErrForbidden
	}
	if !(existingJob.State == models.JobStateWaiting && existingJob.ContractorID == nil) {
		log.Printf("DeleteJob: Invalid state attempt on job %s. State: %s, Contractor: %v", req.ID, existingJob.State, existingJob.ContractorID)
		return ErrInvalidState
	}

	deleteReq := dto.DeleteJobRequest{ID: req.ID}
	err = s.jobRepo.WithTx(tx).Delete(ctx, &deleteReq) // Use tx repo
	if err != nil {
		log.Printf("DeleteJob: Error deleting job %s in repo: %v", req.ID, err)
		return mapRepoError(err, "deleting job")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(ctx); err != nil {
		log.Printf("DeleteJob: Error committing transaction: %v", err)
		return fmt.Errorf("internal error committing job deletion: %w", err)
	}
	// --- End Transaction ---
	return nil
}
