package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
)

type jobService struct {
	jobRepo storage.JobRepository
	userRepo storage.UserRepository
}

// NewJobService creates a new instance of JobService.
func NewJobService(jobRepo storage.JobRepository, userRepo storage.UserRepository) JobService {
	return &jobService{jobRepo: jobRepo, userRepo: userRepo}
}

func (s *jobService) CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error) {
	// EmployerID is already set in the handler from context, passed in req.
	// Add validation if needed (e.g., check if EmployerID exists in user table)
	job, err := s.jobRepo.Create(ctx, req)
	if err != nil {
		log.Printf("JobService: Error creating job: %v", err)
		// Map storage errors if necessary, otherwise return generic internal error
		return nil, fmt.Errorf("internal error creating job: %w", err)
	}
	return job, nil
}

func (s *jobService) GetJobByID(ctx context.Context, req *dto.GetJobByIDRequest) (*models.Job, error) {
	job, err := s.jobRepo.GetByID(ctx, req)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		log.Printf("JobService: Error getting job %s: %v", req.ID, err)
		return nil, fmt.Errorf("internal error getting job: %w", err)
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
	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := s.jobRepo.GetByID(ctx, &getReq)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		log.Printf("UpdateJobDetails: Error fetching job %s: %v", req.JobID, err)
		return nil, fmt.Errorf("internal error fetching job for update: %w", err)
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
	return s.jobRepo.Update(ctx, &updateRepoReq) // Handle potential repo errors (NotFound, Conflict)
}

func (s *jobService) AssignContractor(ctx context.Context, req *dto.AssignContractorRequest) (*models.Job, error) {
	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := s.jobRepo.GetByID(ctx, &getReq)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		log.Printf("AssignContractor: Error fetching job %s: %v", req.JobID, err)
		return nil, fmt.Errorf("internal error fetching job for assignment: %w", err)
	}

	// Authorization: Job state and contractor status
	if !(existingJob.State == models.JobStateWaiting && existingJob.ContractorID == nil) {
		log.Printf("AssignContractor: Invalid state attempt on job %s in state %s with contractor %v", req.JobID, existingJob.State, existingJob.ContractorID)
		return nil, ErrInvalidState
	}

	// Authorization: Role check
	isEmployer := existingJob.EmployerID == req.UserID
	isAssigningSelf := req.ContractorID == req.UserID
	if isEmployer {
		if isAssigningSelf {
			return nil, fmt.Errorf("%w: employer cannot assign themselves", ErrForbidden)
		}
		// Potentially check if req.ContractorID is a valid user ID
	} else { // User is not the employer, must be assigning self
		if !isAssigningSelf {
			return nil, fmt.Errorf("%w: you can only assign yourself", ErrForbidden)
		}
	}

	contractorID := req.ContractorID
	ongoingState := models.JobStateOngoing // Assigning moves state to Ongoing
	updateRepoReq := dto.UpdateJobRequest{
		ID:           req.JobID,
		ContractorID: &contractorID,
		State:        &ongoingState,
	}

	updatedJob, err := s.jobRepo.Update(ctx, &updateRepoReq)
	if errors.Is(err, storage.ErrConflict) { // e.g., invalid contractor ID FK
		return nil, fmt.Errorf("%w: invalid contractor ID", ErrConflict)
	}
	return updatedJob, err // Handle other repo errors (NotFound)
}

func (s *jobService) UnassignContractor(ctx context.Context, req *dto.UnassignContractorRequest) (*models.Job, error) {
	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := s.jobRepo.GetByID(ctx, &getReq)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		log.Printf("UnassignContractor: Error fetching job %s: %v", req.JobID, err)
		return nil, fmt.Errorf("internal error fetching job for unassignment: %w", err)
	}

	// Authorization check: Must be the current contractor and job must be Ongoing
	if !(existingJob.ContractorID != nil && *existingJob.ContractorID == req.UserID && existingJob.State == models.JobStateOngoing) {
		log.Printf("UnassignContractor: Forbidden attempt on job %s by user %s. State: %s, Current Contractor: %v", req.JobID, req.UserID, existingJob.State, existingJob.ContractorID)
		return nil, ErrForbidden // Or ErrInvalidState
	}

	var nilUUID *uuid.UUID
	waitingState := models.JobStateWaiting // Unassigning reverts state to Waiting
	updateRepoReq := dto.UpdateJobRequest{
		ID:           req.JobID,
		ContractorID: nilUUID,
		State:        &waitingState,
	}
	return s.jobRepo.Update(ctx, &updateRepoReq) // Handle repo errors
}

func (s *jobService) UpdateJobState(ctx context.Context, req *dto.UpdateJobStateRequest) (*models.Job, error) {
	getReq := dto.GetJobByIDRequest{ID: req.JobID}
	existingJob, err := s.jobRepo.GetByID(ctx, &getReq)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		log.Printf("UpdateJobState: Error fetching job %s: %v", req.JobID, err)
		return nil, fmt.Errorf("internal error fetching job for state update: %w", err)
	}

	// Authorization check
	isEmployer := existingJob.EmployerID == req.UserID
	isCurrentContractor := existingJob.ContractorID != nil && *existingJob.ContractorID == req.UserID
	if !(isEmployer || isCurrentContractor) {
		log.Printf("UpdateJobState: Forbidden attempt on job %s by user %s. Role: Employer=%t, Contractor=%t", req.JobID, req.UserID, isEmployer, isCurrentContractor)
		return nil, ErrForbidden
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
	return s.jobRepo.Update(ctx, &updateRepoReq) // Handle repo errors
}

func (s *jobService) DeleteJob(ctx context.Context, req *dto.DeleteJobRequest) error {
	getReq := dto.GetJobByIDRequest{ID: req.ID}
	existingJob, err := s.jobRepo.GetByID(ctx, &getReq)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		log.Printf("DeleteJob: Error fetching job %s for delete check: %v", req.ID, err)
		return fmt.Errorf("internal error fetching job for deletion: %w", err)
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
	err = s.jobRepo.Delete(ctx, &deleteReq)
	if errors.Is(err, storage.ErrNotFound) { 
		return ErrNotFound
	}
	return err // Pass up other potential repo errors
}
