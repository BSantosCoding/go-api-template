package services

import (
	"context"
	"fmt"
	"go-api-template/ent/job"
	"go-api-template/ent/jobapplication"
	"go-api-template/internal/api"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/transport/dto"
	"log"

	oapi_middleware "github.com/oapi-codegen/gin-middleware"

	"github.com/google/uuid"
	// Import pgxpool for transaction handling
)

// ApplyToJob creates a new job application for a user to a specific job.
func (sd *ServerDefinition) PostJobsIdApply(ctx context.Context, request api.PostJobsIdApplyRequestObject) (api.PostJobsIdApplyResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Fetch the Job to check its state
	jobReq := dto.GetJobByIDRequest{ID: request.Id}
	jobFound, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, MapRepoError(err, fmt.Sprintf("fetching job %s for application", request.Id))
	}

	// 2. Authorization/Validation
	if jobFound.State != job.StateWaiting || jobFound.ContractorID != uuid.Nil {
		log.Printf("ApplyToJob: Attempt to apply to non-available job %s (State: %s, Contractor: %v)", request.Id, jobFound.State, jobFound.ContractorID)
		return nil, fmt.Errorf("%w: job is not available for applications", ErrInvalidState)
	}
	if jobFound.EmployerID == userID {
		return nil, fmt.Errorf("%w: employer cannot apply to their own job", ErrForbidden)
	}

	existingApplications, err := sd.jobApplicationRepo.GetByJobAndContractor(ctx, &dto.GetByJobAndContractorRequest{UserID: userID, JobID: request.Id})
	if err != nil {
		return nil, MapRepoError(err, fmt.Sprintf("fetching job %s for application", request.Id))
	}
	if len(existingApplications) > 0 {
		log.Printf("ApplyToJob: Attempt to apply to job %s for contractor %s, but already applied", request.Id, userID)
		return nil, fmt.Errorf("%w: already applied to job", ErrConflict)
	}

	// 3. Create the application using the repository
	createReq := dto.CreateJobApplicationRequest{
		JobID:        request.Id,
		ContractorID: userID, // userID from context is the ContractorID
	}
	application, err := sd.jobApplicationRepo.Create(ctx, &createReq)
	if err != nil {
		log.Printf("ApplyToJob: Error creating application in repo: %v", err)
		return nil, MapRepoError(err, "creating application")
	}

	mappedApplication := MapEntJobApplicationToResponse(application)

	return api.PostJobsIdApply201JSONResponse(mappedApplication), nil
}

// AcceptApplication changes application state to Accepted, assigns contractor to job, and sets job state to Ongoing.
func (sd *ServerDefinition) PatchApplicationsIdAccept(ctx context.Context, request api.PatchApplicationsIdAcceptRequestObject) (api.PatchApplicationsIdAcceptResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("AcceptApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	// Use transaction-aware repositories
	txAppRepo := sd.jobApplicationRepo.WithTx(tx)
	txJobRepo := sd.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: request.Id}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		return nil, MapRepoError(err, fmt.Sprintf("fetching application %s within transaction", request.Id))
	}

	// 2. Fetch the Job (within transaction)
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	jobFound, err := txJobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// Should not happen if application exists, but handle defensively
		log.Printf("AcceptApplication: Error fetching job %s within transaction: %v", application.JobID, err)
		return nil, MapRepoError(err, fmt.Sprintf("fetching associated job %s within transaction", application.JobID))
	}

	// 3. Authorization & State Checks
	if jobFound.EmployerID != userID {
		log.Printf("AcceptApplication: Forbidden attempt by user %s on job %s owned by %s", userID, jobFound.ID, jobFound.EmployerID)
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
		return nil, MapRepoError(err, "updating application state")
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
		return nil, MapRepoError(err, "updating job state")
	}

	// 6. Reject other 'Waiting' applications for the same job (within transaction)
	err = txAppRepo.UpdateStateByJobID(ctx, jobFound.ID, jobapplication.StateRejected, &application.ID)
	if err != nil {
		log.Printf("AcceptApplication: Error rejecting other applications for job %s: %v", jobFound.ID, err)
		return nil, MapRepoError(err, "rejecting other applications")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("AcceptApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s accepted, job %s updated to Ongoing with contractor %s", application.ID, updatedJob.ID, contractorID)
	mappedJob := MapEntJobToResponse(updatedJob)

	return api.PatchApplicationsIdAccept200JSONResponse(mappedJob), nil
}

// GetApplicationByID retrieves an application, checking authorization.
// User must be the applicant or the job employer.
func (sd *ServerDefinition) GetApplicationsId(ctx context.Context, request api.GetApplicationsIdRequestObject) (api.GetApplicationsIdResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Fetch the application
	application, err := sd.jobApplicationRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: request.Id})
	if err != nil {
		log.Printf("GetApplicationByID: Error fetching application %s: %v", request.Id, err) // Log before mapping
		return nil, MapRepoError(err, fmt.Sprintf("fetching application %s", request.Id))
	}

	// 2. Fetch the associated job for authorization
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	job, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// This shouldn't happen if the application exists due to FK constraints, but handle defensively
		log.Printf("GetApplicationByID: Error fetching job %s associated with application %s: %v", application.JobID, request.Id, err)
		return nil, MapRepoError(err, fmt.Sprintf("fetching associated job %s", application.JobID))
	}

	// 3. Authorization Check: User must be the applicant or the job employer
	isApplicant := application.ContractorID == userID
	isEmployer := job.EmployerID == userID
	if !isApplicant && !isEmployer {
		log.Printf("GetApplicationByID: Forbidden attempt by user %s on application %s (Applicant: %s, Employer: %s)", userID, request.Id, application.ContractorID, job.EmployerID)
		return nil, ErrForbidden
	}

	mappedApplication := MapEntJobApplicationToResponse(application)

	return api.GetApplicationsId200JSONResponse(mappedApplication), nil
}

// ListApplicationsByContractor retrieves applications for the requesting user.
func (sd *ServerDefinition) GetApplicationsMy(ctx context.Context, request api.GetApplicationsMyRequestObject) (api.GetApplicationsMyResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	applications, err := sd.jobApplicationRepo.ListByContractor(ctx, &dto.ListJobApplicationsByContractorRequest{ContractorID: userID, Limit: *request.Params.Limit, Offset: *request.Params.Offset})
	if err != nil {
		log.Printf("ListApplicationsByContractor: Error listing applications for contractor %s: %v", userID, err)
		return nil, MapRepoError(err, fmt.Sprintf("listing applications for contractor %s", userID))
	}

	mappedApplications := make([]api.DtoJobApplicationResponse, len(applications))
	for i, application := range applications {
		mappedApplications[i] = MapEntJobApplicationToResponse(application)
	}
	return api.GetApplicationsMy200JSONResponse(mappedApplications), nil
}

// ListApplicationsByJob retrieves applications for a specific job, checking authorization.
func (sd *ServerDefinition) GetJobsIdApplications(ctx context.Context, request api.GetJobsIdApplicationsRequestObject) (api.GetJobsIdApplicationsResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// 1. Fetch the job to verify existence and check ownership
	jobReq := dto.GetJobByIDRequest{ID: request.Id}
	job, err := sd.jobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		return nil, MapRepoError(err, fmt.Sprintf("fetching job %s for listing applications", request.Id))
	}

	// 2. Authorization Check: Only the employer can list applications for their job
	if job.EmployerID != userID {
		log.Printf("ListApplicationsByJob: Forbidden attempt by user %s to list applications for job %s owned by %s", userID, request.Id, job.EmployerID)
		return nil, ErrForbidden
	}

	// 3. Call repo method
	applications, err := sd.jobApplicationRepo.ListByJob(ctx, &dto.ListJobApplicationsByJobRequest{JobID: request.Id, Limit: *request.Params.Limit, Offset: *request.Params.Offset})
	if err != nil {
		log.Printf("ListApplicationsByJob: Error listing applications for job %s: %v", request.Id, err)
		return nil, MapRepoError(err, fmt.Sprintf("listing applications for job %s", request.Id))
	}
	mappedApplications := make([]api.DtoJobApplicationResponse, len(applications))
	for i, application := range applications {
		mappedApplications[i] = MapEntJobApplicationToResponse(application)
	}
	return api.GetJobsIdApplications200JSONResponse(mappedApplications), nil
}

// RejectApplication changes application state to Rejected.
func (sd *ServerDefinition) PatchApplicationsIdReject(ctx context.Context, request api.PatchApplicationsIdRejectRequestObject) (api.PatchApplicationsIdRejectResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start (Read-Check-Write pattern) ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("RejectApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback()

	txAppRepo := sd.jobApplicationRepo.WithTx(tx)
	txJobRepo := sd.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: request.Id}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		log.Printf("RejectApplication: Error fetching application %s: %v", request.Id, err) // Log before mapping
		return nil, MapRepoError(err, fmt.Sprintf("fetching application %s", request.Id))
	}

	// 2. Fetch the Job for authorization (within transaction)
	jobReq := dto.GetJobByIDRequest{ID: application.JobID}
	job, err := txJobRepo.GetByID(ctx, &jobReq)
	if err != nil {
		// This shouldn't happen if the application exists, but handle defensively
		log.Printf("RejectApplication: Error fetching job %s for application %s: %v", application.JobID, request.Id, err)
		return nil, MapRepoError(err, fmt.Sprintf("fetching associated job %s", application.JobID))
	}

	// 3. Authorization Check: Only the employer can reject
	if job.EmployerID != userID {
		log.Printf("RejectApplication: Forbidden attempt by user %s on application %s (Job Employer: %s)", userID, request.Id, job.EmployerID)
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
		return nil, MapRepoError(err, "updating application state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("RejectApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing rejection: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s rejected successfully by user %s", updatedApp.ID, userID)
	mappedApplication := MapEntJobApplicationToResponse(updatedApp)

	return api.PatchApplicationsIdReject200JSONResponse(mappedApplication), nil
}

// WithdrawApplication changes application state to Withdrawn.
func (sd *ServerDefinition) PatchApplicationsIdWithdraw(ctx context.Context, request api.PatchApplicationsIdWithdrawRequestObject) (api.PatchApplicationsIdWithdrawResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start (Read-Check-Write pattern) ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("WithdrawApplication: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback()

	txAppRepo := sd.jobApplicationRepo.WithTx(tx)
	// --- End Transaction Setup ---

	// 1. Fetch the Application (within transaction)
	appReq := dto.GetJobApplicationByIDRequest{ID: request.Id}
	application, err := txAppRepo.GetByID(ctx, &appReq)
	if err != nil {
		log.Printf("WithdrawApplication: Error fetching application %s: %v", request.Id, err) // Log before mapping
		return nil, MapRepoError(err, fmt.Sprintf("fetching application %s", request.Id))
	}

	// 2. Authorization Check: Only the applicant (contractor) can withdraw
	if application.ContractorID != userID {
		log.Printf("WithdrawApplication: Forbidden attempt by user %s on application %s owned by %s", userID, request.Id, application.ContractorID)
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
		return nil, MapRepoError(err, "updating application state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("WithdrawApplication: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing withdrawal: %w", err)
	}
	// --- End Transaction ---

	log.Printf("Job application %s withdrawn successfully by user %s", updatedApp.ID, userID)
	mappedApplication := MapEntJobApplicationToResponse(updatedApp)

	return api.PatchApplicationsIdWithdraw200JSONResponse(mappedApplication), nil
}
