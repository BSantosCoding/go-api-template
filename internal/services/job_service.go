package services

import (
	"context"
	"fmt"
	"log"

	"go-api-template/ent/job"
	"go-api-template/internal/api"
	"go-api-template/internal/api/middleware"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	oapi_middleware "github.com/oapi-codegen/gin-middleware"
)

func (sd *ServerDefinition) PostJobs(ctx context.Context, request api.PostJobsRequestObject) (api.PostJobsResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// EmployerID is already set in the handler from context, passed in request.
	job, err := sd.jobRepo.Create(ctx, &dto.CreateJobRequest{Rate: float64(request.Body.Rate), Duration: request.Body.Duration, EmployerID: userID, InvoiceInterval: request.Body.InvoiceInterval})
	if err != nil {
		log.Printf("JobService: Error creating job: %v", err)
		// Map storage errors if necessary (e.g., ErrConflict for FK violation)
		return nil, fmt.Errorf("internal error creating job: %w", err)
	}
	return api.PostJobs201JSONResponse(MapEntJobToResponse(job)), nil
}

func (sd *ServerDefinition) GetJobsId(ctx context.Context, request api.GetJobsIdRequestObject) (api.GetJobsIdResponseObject, error) {
	job, err := sd.jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: request.Id})
	if err != nil {
		log.Printf("JobService: Error getting job %s: %v", request.Id, err)
		return nil, MapRepoError(err, "getting job by ID")
	}

	mappedJob := MapEntJobToResponse(job)

	return api.GetJobsId200JSONResponse(mappedJob), nil
}

func (sd *ServerDefinition) GetJobsAvailable(ctx context.Context, request api.GetJobsAvailableRequestObject) (api.GetJobsAvailableResponseObject, error) {
	jobs, err := sd.jobRepo.ListAvailable(ctx, &dto.ListAvailableJobsRequest{MinRate: ptrFloat64(float64(*request.Params.MinRate)), MaxRate: ptrFloat64(float64(*request.Params.MaxRate)), Limit: *request.Params.Limit, Offset: *request.Params.Offset})
	if err != nil {
		log.Printf("JobService: Error listing available jobs: %v", err)
		return nil, fmt.Errorf("internal error listing available jobs: %w", err)
	}
	mappedJobs := make([]api.DtoJobResponse, len(jobs))
	for i, job := range jobs {
		mappedJobs[i] = MapEntJobToResponse(job)
	}
	return api.GetJobsAvailable200JSONResponse(mappedJobs), nil
}

func (sd *ServerDefinition) GetJobsMyEmployer(ctx context.Context, request api.GetJobsMyEmployerRequestObject) (api.GetJobsMyEmployerResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// EmployerID is set in handler from context and passed in request. (Might change this so it can be overridden to allow listing for other users)
	jobs, err := sd.jobRepo.ListByEmployer(ctx, &dto.ListJobsByEmployerRequest{
		EmployerID: userID,
		Limit:      *request.Params.Limit,
		Offset:     *request.Params.Offset,
		State:      (*job.State)(request.Params.State),
		MinRate:    ptrFloat64(float64(*request.Params.MinRate)),
		MaxRate:    ptrFloat64(float64(*request.Params.MaxRate)),
	})
	if err != nil {
		log.Printf("JobService: Error listing employer jobs for %s: %v", userID, err)
		return nil, fmt.Errorf("internal error listing employer jobs: %w", err)
	}
	mappedJobs := make([]api.DtoJobResponse, len(jobs))
	for i, job := range jobs {
		mappedJobs[i] = MapEntJobToResponse(job)
	}
	return api.GetJobsMyEmployer200JSONResponse(mappedJobs), nil
}

func (sd *ServerDefinition) GetJobsMyContractor(ctx context.Context, request api.GetJobsMyContractorRequestObject) (api.GetJobsMyContractorResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// ContractorID is set in handler from context and passed in request. (Might change this so it can be overridden to allow listing for other users)
	jobs, err := sd.jobRepo.ListByContractor(ctx, &dto.ListJobsByContractorRequest{
		ContractorID: userID,
		Limit:        *request.Params.Limit,
		Offset:       *request.Params.Offset,
		State:        (*job.State)(request.Params.State),
		MinRate:      ptrFloat64(float64(*request.Params.MinRate)),
		MaxRate:      ptrFloat64(float64(*request.Params.MaxRate)),
	})
	if err != nil {
		log.Printf("JobService: Error listing contractor jobs for %s: %v", userID, err)
		return nil, fmt.Errorf("internal error listing contractor jobs: %w", err)
	}
	mappedJobs := make([]api.DtoJobResponse, len(jobs))
	for i, job := range jobs {
		mappedJobs[i] = MapEntJobToResponse(job)
	}
	return api.GetJobsMyContractor200JSONResponse(mappedJobs), nil
}

func (sd *ServerDefinition) PatchJobsIdDetails(ctx context.Context, request api.PatchJobsIdDetailsRequestObject) (api.PatchJobsIdDetailsResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("UpdateJobDetails: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	// Use transaction-aware repository
	txJobRepo := sd.jobRepo.WithTx(tx)
	// --- End Transaction Setup ---

	getReq := dto.GetJobByIDRequest{ID: request.Id}
	existingJob, err := txJobRepo.GetByID(ctx, &getReq) // Use txJobRepo
	if err != nil {
		log.Printf("UpdateJobDetails: Error fetching job %s: %v", request.Id, err)
		return nil, MapRepoError(err, "fetching job for update")
	}

	// Authorization & State Check
	if !(userID == existingJob.EmployerID && existingJob.State == job.StateWaiting && existingJob.ContractorID == uuid.Nil) {
		log.Printf("UpdateJobDetails: Forbidden attempt on job %s by user %sd. State: %s, Contractor: %v", request.Id, userID, existingJob.State, existingJob.ContractorID)
		return nil, ErrForbidden // Or ErrInvalidState
	}

	updateRepoReq := dto.UpdateJobRequest{
		ID:       request.Id,
		Rate:     ptrFloat64(float64(*request.Body.Rate)),
		Duration: request.Body.Duration,
	}
	updatedJob, err := txJobRepo.Update(ctx, &updateRepoReq) // Use txJobRepo
	if err != nil {
		log.Printf("UpdateJobDetails: Error updating job %s in repo: %v", request.Id, err)
		return nil, MapRepoError(err, "updating job details")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("UpdateJobDetails: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---
	mappedJob := MapEntJobToResponse(updatedJob)

	return api.PatchJobsIdDetails200JSONResponse(mappedJob), nil
}

func (sd *ServerDefinition) PatchJobsIdState(ctx context.Context, request api.PatchJobsIdStateRequestObject) (api.PatchJobsIdStateResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("UpdateJobState: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	getReq := dto.GetJobByIDRequest{ID: request.Id}
	existingJob, err := sd.jobRepo.WithTx(tx).GetByID(ctx, &getReq) // Use tx repo
	if err != nil {
		log.Printf("UpdateJobState: Error fetching job %s: %v", request.Id, err)
		return nil, MapRepoError(err, "fetching job for state update")
	}

	// Authorization check
	isEmployer := existingJob.EmployerID == userID
	isCurrentContractor := existingJob.ContractorID != uuid.Nil && existingJob.ContractorID == userID
	if !(isEmployer || isCurrentContractor) {
		log.Printf("UpdateJobState: Forbidden attempt on job %s by user %sd. Role: Employer=%t, Contractor=%t", request.Id, userID, isEmployer, isCurrentContractor)
		return nil, ErrForbidden
	}

	// Prevent manual state change to Ongoing - this should only happen via AcceptApplication
	if job.State(request.Body.State) == job.StateOngoing && existingJob.State == job.StateWaiting {
		log.Printf("UpdateJobState: Forbidden attempt to manually set job %s to Ongoing by user %sd.", request.Id, userID)
		return nil, fmt.Errorf("%w: cannot manually set state to Ongoing, use AcceptApplication", ErrInvalidTransition)
	}

	// Validation: Check state transition
	if !isValidJobStateTransition(existingJob.State, job.State(request.Body.State)) {
		return nil, fmt.Errorf("%w: from %s to %s", ErrInvalidTransition, existingJob.State, request.Body.State)
	}

	newState := job.State(request.Body.State)
	updateRepoReq := dto.UpdateJobRequest{
		ID:    request.Id,
		State: &newState,
	}
	updatedJob, err := sd.jobRepo.WithTx(tx).Update(ctx, &updateRepoReq) // Use tx repo
	if err != nil {
		log.Printf("UpdateJobState: Error updating job state %s in repo: %v", request.Id, err)
		return nil, MapRepoError(err, "updating job state")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("UpdateJobState: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing changes: %w", err)
	}
	// --- End Transaction ---

	mappedJob := MapEntJobToResponse(updatedJob)

	return api.PatchJobsIdState200JSONResponse(mappedJob), nil
}

func (sd *ServerDefinition) DeleteJobsId(ctx context.Context, request api.DeleteJobsIdRequestObject) (api.DeleteJobsIdResponseObject, error) {
	c := oapi_middleware.GetGinContext(ctx)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		return nil, err
	}

	// --- Transaction Start ---
	tx, err := sd.db.Tx(ctx)
	if err != nil {
		log.Printf("DeleteJob: Error beginning transaction: %v", err)
		return nil, fmt.Errorf("internal error starting transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if anything fails

	getReq := dto.GetJobByIDRequest{ID: request.Id}
	existingJob, err := sd.jobRepo.WithTx(tx).GetByID(ctx, &getReq) // Use tx repo
	if err != nil {
		log.Printf("DeleteJob: Error fetching job %s for delete check: %v", request.Id, err)
		return nil, MapRepoError(err, "fetching job for delete check")
	}

	// Authorization Check
	if existingJob.EmployerID != userID {
		log.Printf("DeleteJob: Forbidden attempt on job %s by non-employer user %s", request.Id, userID)
		return nil, ErrForbidden
	}
	if !(existingJob.State == job.StateWaiting && existingJob.ContractorID == uuid.Nil) {
		log.Printf("DeleteJob: Invalid state attempt on job %sd. State: %s, Contractor: %v", request.Id, existingJob.State, existingJob.ContractorID)
		return nil, ErrInvalidState
	}

	deleteReq := dto.DeleteJobRequest{ID: request.Id}
	err = sd.jobRepo.WithTx(tx).Delete(ctx, &deleteReq) // Use tx repo
	if err != nil {
		log.Printf("DeleteJob: Error deleting job %s in repo: %v", request.Id, err)
		return nil, MapRepoError(err, "deleting job")
	}

	// --- Commit Transaction ---
	if err := tx.Commit(); err != nil {
		log.Printf("DeleteJob: Error committing transaction: %v", err)
		return nil, fmt.Errorf("internal error committing job deletion: %w", err)
	}
	// --- End Transaction ---
	return api.DeleteJobsId204Response{}, nil
}
