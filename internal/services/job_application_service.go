package services

import (
	"context"
	"fmt"
	"go-api-template/ent"
	"go-api-template/ent/job"
	"go-api-template/ent/jobapplication"
	"go-api-template/internal/storage"
	"go-api-template/internal/storage/postgres"
	"go-api-template/internal/transport/dto"
	"log"

	"github.com/google/uuid"
	// Import pgxpool for transaction handling
)

type jobApplicationService struct {
	appRepo storage.JobApplicationRepository
	jobRepo storage.JobRepository
	db      *ent.Client
}

// NewJobApplicationService creates a new instance of JobApplicationService.
func NewJobApplicationService(db *ent.Client) JobApplicationService {
	return &jobApplicationService{
		appRepo: postgres.NewJobApplicationRepo(db),
		jobRepo: postgres.NewJobRepo(db),
		db:      db,
	}
}

// ApplyToJob creates a new job application for a user to a specific job.
func (s *jobApplicationService) ApplyToJob(ctx context.Context, req *dto.ApplyToJobRequest) (*ent.JobApplication, error) {
	// 1. Fetch the Job to check its state
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	jobFound, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, mapRepoError(err, fmt.Sprintf("fetching job %s for application", req.JobID))
	}

	// 2. Authorization/Validation
	if jobFound.State != job.StateWaiting || jobFound.ContractorID != uuid.Nil {
		log.Printf("ApplyToJob: Attempt to apply to non-available job %s (State: %s, Contractor: %v)", req.JobID, jobFound.State, jobFound.ContractorID)
		return nil, fmt.Errorf("%w: job is not available for applications", ErrInvalidState)
	}
	if jobFound.EmployerID == req.ContractorID {
		return nil, fmt.Errorf("%w: employer cannot apply to their own job", ErrForbidden)
	}

	existingApplications, err := s.appRepo.GetByJobAndContractor(ctx, &dto.GetByJobAndContractorRequest{UserID: req.ContractorID, JobID: req.JobID})
	if err != nil {
		return nil, mapRepoError(err, fmt.Sprintf("fetching job %s for application", req.JobID))
	}
	if len(existingApplications) > 0 {
		log.Printf("ApplyToJob: Attempt to apply to job %s for contractor %s, but already applied", req.JobID, req.ContractorID)
		return nil, fmt.Errorf("%w: already applied to job", ErrConflict)
	}

	// 3. Create the application using the repository
	createReq := dto.CreateJobApplicationRequest{
		JobID:        req.JobID,
		ContractorID: req.ContractorID, // UserID from context is the ContractorID
	}
	application, err := s.appRepo.Create(ctx, &createReq)
	if err != nil {
		log.Printf("ApplyToJob: Error creating application in repo: %v", err)
		return nil, mapRepoError(err, "creating application")
	}

	return application, nil
}

// AcceptApplication changes application state to Accepted, assigns contractor to job, and sets job state to Ongoing.
func (s *jobApplicationService) AcceptApplication(ctx context.Context, req *dto.AcceptApplicationRequest) (*ent.Job, error) {
	// --- Transaction Start ---
	tx, err := s.db.Tx(ctx)
	if err != nil {
		log.Printf("AcceptApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	// Use transaction-aware repositories
	txAppRepo := s.appRepo.WithTx(tx)
	txJobRepo := s.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: req.ApplicationID}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		return nil, mapRepoError(err, fmt.Sprintf("fetching application %s within transaction", req.ApplicationID))
	}

	// 2. Fetch the Job (within transaction)
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	jobFound, err := txJobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// Should not happen if application exists, but handle defensively
		log.Printf("AcceptApplication: Error fetching job %s within transaction: %v", application.JobID, err)
		return nil, mapRepoError(err, fmt.Sprintf("fetching associated job %s within transaction", application.JobID))
	}

	// 3. Authorization & State Checks
	if jobFound.EmployerID != req.UserID {
		log.Printf("AcceptApplication: Forbidden attempt by user %s on job %s owned by %s", req.UserID, jobFound.ID, jobFound.EmployerID)
		return nil, ErrForbidden
	}
	if jobFound.State != job.StateWaiting || jobFound.ContractorID != uuid.Nil {
		log.Printf("AcceptApplication: Attempt to accept application for non-available job %s (State: %s, Contractor: %v)", jobFound.ID, jobFound.State, jobFound.ContractorID)
		return nil, fmt.Errorf("%w: job is not in a state to accept applications", ErrInvalidState)
	}
	if application.State != jobapplication.StateWaiting {
		log.Printf("AcceptApplication: Attempt to accept non-waiting application %s (State: %s)", application.ID, application.State)
		return nil, fmt.Errorf("%w: application is not in 'Waiting' state", ErrInvalidState)
	}

	// 4. Update Application State (within transaction)
	updateAppReq := dto.UpdateJobApplicationStateRequest{ID: application.ID, State: jobapplication.StateAccepted}
	_, err = txAppRepo.UpdateState(ctx, &updateAppReq)
	if err != nil {
		log.Printf("AcceptApplication: Error updating application state for %s: %v", application.ID, err)
		return nil, mapRepoError(err, "updating application state")
	}

	// 5. Update Job State and Assign Contractor (within transaction)
	contractorID := application.ContractorID
	newState := job.StateOngoing
	updateJobReq := dto.UpdateJobRequest{
		ID:           jobFound.ID,
		ContractorID: &contractorID,
		State:        &newState,
	}
	updatedJob, err := txJobRepo.Update(ctx, &updateJobReq)
	if err != nil {
		log.Printf("AcceptApplication: Error updating job %s: %v", jobFound.ID, err)
		return nil, mapRepoError(err, "updating job state")
	}

	// 6. Reject other 'Waiting' applications for the same job (within transaction)
	err = txAppRepo.UpdateStateByJobID(ctx, jobFound.ID, jobapplication.StateRejected, &application.ID)
	if err != nil {
		log.Printf("AcceptApplication: Error rejecting other applications for job %s: %v", jobFound.ID, err)
		return nil, mapRepoError(err, "rejecting other applications")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("AcceptApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s accepted, job %s updated to Ongoing with contractor %s", application.ID, updatedJob.ID, contractorID)
	return updatedJob, nil
}

// GetApplicationByID retrieves an application, checking authorization.
// User must be the applicant or the job employer.
func (s *jobApplicationService) GetApplicationByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*ent.JobApplication, error) {
	// 1. Fetch the application
	application, err := s.appRepo.GetByID(ctx, req)
	if err != nil {
		log.Printf("GetApplicationByID: Error fetching application %s: %v", req.ID, err) // Log before mapping
		return nil, mapRepoError(err, fmt.Sprintf("fetching application %s", req.ID))
	}

	// 2. Fetch the associated job for authorization
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// This shouldn't happen if the application exists due to FK constraints, but handle defensively
		log.Printf("GetApplicationByID: Error fetching job %s associated with application %s: %v", application.JobID, req.ID, err)
		return nil, mapRepoError(err, fmt.Sprintf("fetching associated job %s", application.JobID))
	}

	// 3. Authorization Check: User must be the applicant or the job employer
	isApplicant := application.ContractorID == req.UserID
	isEmployer := job.EmployerID == req.UserID
	if !isApplicant && !isEmployer {
		log.Printf("GetApplicationByID: Forbidden attempt by user %s on application %s (Applicant: %s, Employer: %s)", req.UserID, req.ID, application.ContractorID, job.EmployerID)
		return nil, ErrForbidden
	}

	return application, nil
}

// ListApplicationsByContractor retrieves applications for the requesting user.
func (s *jobApplicationService) ListApplicationsByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]*ent.JobApplication, error) {
	applications, err := s.appRepo.ListByContractor(ctx, req)
	if err != nil {
		log.Printf("ListApplicationsByContractor: Error listing applications for contractor %s: %v", req.ContractorID, err)
		return nil, mapRepoError(err, fmt.Sprintf("listing applications for contractor %s", req.ContractorID))
	}
	return applications, nil
}

// ListApplicationsByJob retrieves applications for a specific job, checking authorization.
func (s *jobApplicationService) ListApplicationsByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]*ent.JobApplication, error) {
	// 1. Fetch the job to verify existence and check ownership
	jobReq := dto.GetJobByIDRequest{ID: req.JobID}
	job, err := s.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, mapRepoError(err, fmt.Sprintf("fetching job %s for listing applications", req.JobID))
	}

	// 2. Authorization Check: Only the employer can list applications for their job
	if job.EmployerID != req.UserID {
		log.Printf("ListApplicationsByJob: Forbidden attempt by user %s to list applications for job %s owned by %s", req.UserID, req.JobID, job.EmployerID)
		return nil, ErrForbidden
	}

	// 3. Call repo method
	applications, err := s.appRepo.ListByJob(ctx, req)
	if err != nil {
		log.Printf("ListApplicationsByJob: Error listing applications for job %s: %v", req.JobID, err)
		return nil, mapRepoError(err, fmt.Sprintf("listing applications for job %s", req.JobID))
	}
	return applications, nil
}

// RejectApplication changes application state to Rejected.
func (s *jobApplicationService) RejectApplication(ctx context.Context, req *dto.RejectApplicationRequest) (*ent.JobApplication, error) {
	// --- Transaction Start (Read-Check-Write pattern) ---
	tx, err := s.db.Tx(ctx)
	if err != nil {
		log.Printf("RejectApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback()

	txAppRepo := s.appRepo.WithTx(tx)
	txJobRepo := s.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: req.ApplicationID}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		log.Printf("RejectApplication: Error fetching application %s: %v", req.ApplicationID, err) // Log before mapping
		return nil, mapRepoError(err, fmt.Sprintf("fetching application %s", req.ApplicationID))
	}

	// 2. Fetch the Job for authorization (within transaction)
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	job, err := txJobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// This shouldn't happen if the application exists, but handle defensively
		log.Printf("RejectApplication: Error fetching job %s for application %s: %v", application.JobID, req.ApplicationID, err)
		return nil, mapRepoError(err, fmt.Sprintf("fetching associated job %s", application.JobID))
	}

	// 3. Authorization Check: Only the employer can reject
	if job.EmployerID != req.UserID {
		log.Printf("RejectApplication: Forbidden attempt by user %s on application %s (Job Employer: %s)", req.UserID, req.ApplicationID, job.EmployerID)
		return nil, ErrForbidden
	}

	// 4. State Check: Can only reject 'Waiting' applications
	if application.State != jobapplication.StateWaiting {
		log.Printf("RejectApplication: Attempt to reject non-waiting application %s (State: %s)", application.ID, application.State)
		return nil, fmt.Errorf("%w: application is not in 'Waiting' state, current state: %s", ErrInvalidState, application.State)
	}

	// 5. Update Application State (within transaction)
	updateReq := dto.UpdateJobApplicationStateRequest{ID: application.ID, State: jobapplication.StateRejected}
	updatedApp, err := txAppRepo.UpdateState(ctx, &updateReq)
	if err != nil {
		log.Printf("RejectApplication: Error updating application state for %s: %v", application.ID, err)
		return nil, mapRepoError(err, "updating application state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("RejectApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing rejection: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s rejected successfully by user %s", updatedApp.ID, req.UserID)
	return updatedApp, nil
}

// WithdrawApplication changes application state to Withdrawn.
func (s *jobApplicationService) WithdrawApplication(ctx context.Context, req *dto.WithdrawApplicationRequest) (*ent.JobApplication, error) {
	// --- Transaction Start (Read-Check-Write pattern) ---
	tx, err := s.db.Tx(ctx)
	if err != nil {
		log.Printf("WithdrawApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback()

	txAppRepo := s.appRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: req.ApplicationID}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		log.Printf("WithdrawApplication: Error fetching application %s: %v", req.ApplicationID, err) // Log before mapping
		return nil, mapRepoError(err, fmt.Sprintf("fetching application %s", req.ApplicationID))
	}

	// 2. Authorization Check: Only the applicant (contractor) can withdraw
	if application.ContractorID != req.UserID {
		log.Printf("WithdrawApplication: Forbidden attempt by user %s on application %s owned by %s", req.UserID, req.ApplicationID, application.ContractorID)
		return nil, ErrForbidden
	}

	// 3. State Check: Can only withdraw 'Waiting' applications
	if application.State != jobapplication.StateWaiting {
		log.Printf("WithdrawApplication: Attempt to withdraw non-waiting application %s (State: %s)", application.ID, application.State)
		return nil, fmt.Errorf("%w: application is not in 'Waiting' state, current state: %s", ErrInvalidState, application.State)
	}

	// 4. Update Application State (within transaction)
	updateReq := dto.UpdateJobApplicationStateRequest{ID: application.ID, State: jobapplication.StateWithdrawn}
	updatedApp, err := txAppRepo.UpdateState(ctx, &updateReq)
	if err != nil {
		log.Printf("WithdrawApplication: Error updating application state for %s: %v", application.ID, err)
		return nil, mapRepoError(err, "updating application state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("WithdrawApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing withdrawal: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s withdrawn successfully by user %s", updatedApp.ID, req.UserID)
	return updatedApp, nil
}
