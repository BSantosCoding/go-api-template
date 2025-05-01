package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mock_storage "go-api-template/internal/mocks" // Assuming mocks are generated here
	"go-api-template/internal/models"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"
	"go-api-template/internal/transport/dto"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a pointer to a UUID
func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

// Helper to create a pointer to a float64
func ptrFloat64(f float64) *float64 { return &f }

// Helper to create a pointer to an int
func ptrInt(i int) *int { return &i }

// Helper to create a pointer to a JobState
func ptrJobState(s models.JobState) *models.JobState { return &s }

func setupJobServiceTest(t *testing.T) (context.Context, services.JobService, *mock_storage.MockJobRepository, *mock_storage.MockUserRepository, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockJobRepo := mock_storage.NewMockJobRepository(ctrl)
	mockUserRepo := mock_storage.NewMockUserRepository(ctrl) // Needed for constructor
	jobService := services.NewJobService(mockJobRepo, mockUserRepo)
	ctx := context.Background()
	return ctx, jobService, mockJobRepo, mockUserRepo, ctrl
}

func TestJobService_CreateJob_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	employerID := uuid.New()
	req := &dto.CreateJobRequest{
		Rate:            100.50,
		Duration:        40,
		InvoiceInterval: 10,
		EmployerID:      employerID, // Set by handler in real scenario
	}

	expectedJob := &models.Job{
		ID:              uuid.New(),
		Rate:            req.Rate,
		Duration:        req.Duration,
		InvoiceInterval: req.InvoiceInterval,
		EmployerID:      req.EmployerID,
		State:           models.JobStateWaiting,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	mockJobRepo.EXPECT().Create(ctx, req).Return(expectedJob, nil).Times(1)

	job, err := jobService.CreateJob(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedJob, job)
}

func TestJobService_CreateJob_RepoError(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	req := &dto.CreateJobRequest{
		Rate:            100.50,
		Duration:        40,
		InvoiceInterval: 10,
		EmployerID:      uuid.New(),
	}
	repoErr := errors.New("db connection failed")

	mockJobRepo.EXPECT().Create(ctx, req).Return(nil, repoErr).Times(1)

	_, err := jobService.CreateJob(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error creating job")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_GetJobByID_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.GetJobByIDRequest{ID: jobID}
	expectedJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, req).Return(expectedJob, nil).Times(1)

	job, err := jobService.GetJobByID(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedJob, job)
}

func TestJobService_GetJobByID_NotFound(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.GetJobByIDRequest{ID: jobID}

	mockJobRepo.EXPECT().GetByID(ctx, req).Return(nil, storage.ErrNotFound).Times(1)

	_, err := jobService.GetJobByID(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_GetJobByID_RepoError(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.GetJobByIDRequest{ID: jobID}
	repoErr := errors.New("db read error")

	mockJobRepo.EXPECT().GetByID(ctx, req).Return(nil, repoErr).Times(1)

	_, err := jobService.GetJobByID(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error getting job")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_ListAvailableJobs_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	req := &dto.ListAvailableJobsRequest{Limit: 5, Offset: 0}
	expectedJobs := []models.Job{
		{ID: uuid.New(), State: models.JobStateWaiting, EmployerID: uuid.New()},
		{ID: uuid.New(), State: models.JobStateWaiting, EmployerID: uuid.New()},
	}

	mockJobRepo.EXPECT().ListAvailable(ctx, req).Return(expectedJobs, nil).Times(1)

	jobs, err := jobService.ListAvailableJobs(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedJobs, jobs)
}

func TestJobService_ListAvailableJobs_RepoError(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	req := &dto.ListAvailableJobsRequest{Limit: 10, Offset: 0}
	repoErr := errors.New("db query failed")

	mockJobRepo.EXPECT().ListAvailable(ctx, req).Return(nil, repoErr).Times(1)

	_, err := jobService.ListAvailableJobs(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error listing available jobs")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_ListJobsByEmployer_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	employerID := uuid.New()
	req := &dto.ListJobsByEmployerRequest{EmployerID: employerID, Limit: 5, Offset: 0}
	expectedJobs := []models.Job{
		{ID: uuid.New(), EmployerID: employerID, State: models.JobStateWaiting},
		{ID: uuid.New(), EmployerID: employerID, State: models.JobStateOngoing, ContractorID: ptrUUID(uuid.New())},
	}

	mockJobRepo.EXPECT().ListByEmployer(ctx, req).Return(expectedJobs, nil).Times(1)

	jobs, err := jobService.ListJobsByEmployer(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedJobs, jobs)
}

func TestJobService_ListJobsByEmployer_RepoError(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	employerID := uuid.New()
	req := &dto.ListJobsByEmployerRequest{EmployerID: employerID, Limit: 10, Offset: 0}
	repoErr := errors.New("db query failed")

	mockJobRepo.EXPECT().ListByEmployer(ctx, req).Return(nil, repoErr).Times(1)

	_, err := jobService.ListJobsByEmployer(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error listing employer jobs")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_ListJobsByContractor_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	contractorID := uuid.New()
	req := &dto.ListJobsByContractorRequest{ContractorID: contractorID, Limit: 5, Offset: 0}
	expectedJobs := []models.Job{
		{ID: uuid.New(), EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: ptrUUID(contractorID)},
		{ID: uuid.New(), EmployerID: uuid.New(), State: models.JobStateComplete, ContractorID: ptrUUID(contractorID)},
	}

	mockJobRepo.EXPECT().ListByContractor(ctx, req).Return(expectedJobs, nil).Times(1)

	jobs, err := jobService.ListJobsByContractor(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedJobs, jobs)
}

func TestJobService_ListJobsByContractor_RepoError(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	contractorID := uuid.New()
	req := &dto.ListJobsByContractorRequest{ContractorID: contractorID, Limit: 10, Offset: 0}
	repoErr := errors.New("db query failed")

	mockJobRepo.EXPECT().ListByContractor(ctx, req).Return(nil, repoErr).Times(1)

	_, err := jobService.ListJobsByContractor(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error listing contractor jobs")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_UpdateJobDetails_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{
		JobID:    jobID,
		UserID:   employerID,
		Rate:     ptrFloat64(120.0),
		Duration: ptrInt(50),
	}

	existingJob := &models.Job{
		ID:         jobID,
		EmployerID: employerID,
		State:      models.JobStateWaiting, // Correct state
		ContractorID: nil, // No contractor
		Rate:       100.0,
		Duration:   40,
	}

	updatedJob := &models.Job{
		ID:         jobID,
		EmployerID: employerID,
		State:      models.JobStateWaiting,
		ContractorID: nil,
		Rate:       *req.Rate,
		Duration:   *req.Duration,
		UpdatedAt:  time.Now(),
	}

	// Mock GetByID first
	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	// Mock Update
	expectedUpdateReq := &dto.UpdateJobRequest{
		ID:       jobID,
		Rate:     req.Rate,
		Duration: req.Duration,
	}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.UpdateJobDetails(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_UpdateJobDetails_NotFound(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: uuid.New(), Rate: ptrFloat64(110.0)}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, storage.ErrNotFound).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_UpdateJobDetails_Forbidden_WrongUser(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	wrongUserID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: wrongUserID, Rate: ptrFloat64(110.0)}

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_UpdateJobDetails_Forbidden_WrongState(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: employerID, Rate: ptrFloat64(110.0)}

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing} // Wrong state

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden)) // Service maps this specific check to Forbidden
}

func TestJobService_UpdateJobDetails_Forbidden_ContractorAssigned(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: employerID, Rate: ptrFloat64(110.0)}

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting, ContractorID: &contractorID} // Contractor assigned

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden)) // Service maps this specific check to Forbidden
}

func TestJobService_UpdateJobDetails_RepoError_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: uuid.New(), Rate: ptrFloat64(110.0)}
	repoErr := errors.New("db read failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, repoErr).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error fetching job for update")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_UpdateJobDetails_RepoError_Update(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.UpdateJobDetailsRequest{JobID: jobID, UserID: employerID, Rate: ptrFloat64(110.0)}
	repoErr := errors.New("db write failed")

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)
	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, Rate: req.Rate, Duration: req.Duration}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(nil, repoErr).Times(1)

	_, err := jobService.UpdateJobDetails(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Update error is passed through
}

func TestJobService_AssignContractor_Success_EmployerAssigns(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.AssignContractorRequest{
		JobID:        jobID,
		UserID:       employerID, // User making request is employer
		ContractorID: contractorID, // Assigning another user
	}

	existingJob := &models.Job{
		ID:           jobID,
		EmployerID:   employerID,
		State:        models.JobStateWaiting, // Correct state
		ContractorID: nil,                   // No contractor
	}

	updatedJob := &models.Job{
		ID:           jobID,
		EmployerID:   employerID,
		State:        models.JobStateOngoing, // State changes
		ContractorID: &contractorID,        // Contractor set
		UpdatedAt:    time.Now(),
	}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{
		ID:           jobID,
		ContractorID: &contractorID,
		State:        ptrJobState(models.JobStateOngoing),
	}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.AssignContractor(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_AssignContractor_Success_ContractorAssignsSelf(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.AssignContractorRequest{
		JobID:        jobID,
		UserID:       contractorID, // User making request is the contractor
		ContractorID: contractorID, // Assigning self
	}

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting, ContractorID: nil}
	updatedJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing, ContractorID: &contractorID, UpdatedAt: time.Now()}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, ContractorID: &contractorID, State: ptrJobState(models.JobStateOngoing)}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.AssignContractor(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_AssignContractor_NotFound(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: uuid.New(), ContractorID: uuid.New()}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, storage.ErrNotFound).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_AssignContractor_InvalidState_NotWaiting(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: uuid.New(), ContractorID: uuid.New()}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateOngoing} // Wrong state

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidState))
}

func TestJobService_AssignContractor_InvalidState_ContractorExists(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	contractorID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: uuid.New(), ContractorID: uuid.New()}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateWaiting, ContractorID: &contractorID} // Contractor exists

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidState))
}

func TestJobService_AssignContractor_Forbidden_EmployerAssignsSelf(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: employerID, ContractorID: employerID} // Employer assigning self
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
	assert.Contains(t, err.Error(), "employer cannot assign themselves")
}

func TestJobService_AssignContractor_Forbidden_NonEmployerAssignsOther(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	requestingUserID := uuid.New()
	targetContractorID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: requestingUserID, ContractorID: targetContractorID} // Non-employer assigning someone else
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
	assert.Contains(t, err.Error(), "you can only assign yourself")
}

func TestJobService_AssignContractor_Conflict_Update(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: employerID, ContractorID: contractorID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, ContractorID: &contractorID, State: ptrJobState(models.JobStateOngoing)}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(nil, storage.ErrConflict).Times(1) // Simulate FK violation etc.

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrConflict))
	assert.Contains(t, err.Error(), "invalid contractor ID")
}

func TestJobService_AssignContractor_RepoError_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: uuid.New(), ContractorID: uuid.New()}
	repoErr := errors.New("db read failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, repoErr).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error fetching job for assignment")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_AssignContractor_RepoError_Update(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.AssignContractorRequest{JobID: jobID, UserID: employerID, ContractorID: contractorID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}
	repoErr := errors.New("db write failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, ContractorID: &contractorID, State: ptrJobState(models.JobStateOngoing)}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(nil, repoErr).Times(1)

	_, err := jobService.AssignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Update error passed through
}

func TestJobService_UnassignContractor_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	contractorID := uuid.New()
	req := &dto.UnassignContractorRequest{
		JobID:  jobID,
		UserID: contractorID, // User making request is the current contractor
	}

	existingJob := &models.Job{
		ID:           jobID,
		EmployerID:   uuid.New(),
		State:        models.JobStateOngoing, // Correct state
		ContractorID: &contractorID,        // Correct contractor
	}

	updatedJob := &models.Job{
		ID:           jobID,
		EmployerID:   existingJob.EmployerID,
		State:        models.JobStateWaiting, // State reverts
		ContractorID: nil,                   // Contractor removed
		UpdatedAt:    time.Now(),
	}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	var nilUUID *uuid.UUID // Need explicit nil pointer for expectation
	expectedUpdateReq := &dto.UpdateJobRequest{
		ID:           jobID,
		ContractorID: nilUUID,
		State:        ptrJobState(models.JobStateWaiting),
	}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.UnassignContractor(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_UnassignContractor_NotFound(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: uuid.New()}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, storage.ErrNotFound).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_UnassignContractor_Forbidden_WrongUser(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	currentContractorID := uuid.New()
	wrongUserID := uuid.New()
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: wrongUserID}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: &currentContractorID}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_UnassignContractor_Forbidden_NoContractor(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	userID := uuid.New() // Doesn't matter who tries if no contractor
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: userID}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: nil} // No contractor

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_UnassignContractor_Forbidden_WrongState(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	contractorID := uuid.New()
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: contractorID}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateWaiting, ContractorID: &contractorID} // Wrong state

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_UnassignContractor_RepoError_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: uuid.New()}
	repoErr := errors.New("db read failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, repoErr).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error fetching job for unassignment")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_UnassignContractor_RepoError_Update(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	contractorID := uuid.New()
	req := &dto.UnassignContractorRequest{JobID: jobID, UserID: contractorID}
	existingJob := &models.Job{ID: jobID, EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: &contractorID}
	repoErr := errors.New("db write failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	var nilUUID *uuid.UUID
	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, ContractorID: nilUUID, State: ptrJobState(models.JobStateWaiting)}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(nil, repoErr).Times(1)

	_, err := jobService.UnassignContractor(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Update error passed through
}

func TestJobService_UpdateJobState_Success_Employer(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	newState := models.JobStateComplete
	req := &dto.UpdateJobStateRequest{
		JobID:  jobID,
		UserID: employerID, // Employer making request
		State:  newState,
	}

	existingJob := &models.Job{
		ID:           jobID,
		EmployerID:   employerID,
		State:        models.JobStateOngoing, // Valid previous state
		ContractorID: &contractorID,
	}

	updatedJob := &models.Job{
		ID:           jobID,
		EmployerID:   employerID,
		State:        newState, // State updated
		ContractorID: &contractorID,
		UpdatedAt:    time.Now(),
	}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{
		ID:    jobID,
		State: &newState,
	}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.UpdateJobState(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_UpdateJobState_Success_Contractor(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	newState := models.JobStateComplete
	req := &dto.UpdateJobStateRequest{
		JobID:  jobID,
		UserID: contractorID, // Contractor making request
		State:  newState,
	}

	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing, ContractorID: &contractorID}
	updatedJob := &models.Job{ID: jobID, EmployerID: employerID, State: newState, ContractorID: &contractorID, UpdatedAt: time.Now()}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, State: &newState}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(updatedJob, nil).Times(1)

	job, err := jobService.UpdateJobState(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, updatedJob, job)
}

func TestJobService_UpdateJobState_NotFound(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UpdateJobStateRequest{JobID: jobID, UserID: uuid.New(), State: models.JobStateComplete}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, storage.ErrNotFound).Times(1)

	_, err := jobService.UpdateJobState(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_UpdateJobState_Forbidden_WrongUser(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	wrongUserID := uuid.New()
	req := &dto.UpdateJobStateRequest{JobID: jobID, UserID: wrongUserID, State: models.JobStateComplete}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing, ContractorID: &contractorID}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UpdateJobState(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_UpdateJobState_InvalidTransition(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.UpdateJobStateRequest{JobID: jobID, UserID: employerID, State: models.JobStateWaiting} // Invalid: Complete -> Waiting
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateComplete}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	_, err := jobService.UpdateJobState(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidTransition))
	assert.Contains(t, err.Error(), "from Complete to Waiting")
}

func TestJobService_UpdateJobState_RepoError_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.UpdateJobStateRequest{JobID: jobID, UserID: uuid.New(), State: models.JobStateComplete}
	repoErr := errors.New("db read failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, repoErr).Times(1)

	_, err := jobService.UpdateJobState(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error fetching job for state update")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_UpdateJobState_RepoError_Update(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	newState := models.JobStateComplete
	req := &dto.UpdateJobStateRequest{JobID: jobID, UserID: employerID, State: newState}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing}
	repoErr := errors.New("db write failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	expectedUpdateReq := &dto.UpdateJobRequest{ID: jobID, State: &newState}
	mockJobRepo.EXPECT().Update(ctx, expectedUpdateReq).Return(nil, repoErr).Times(1)

	_, err := jobService.UpdateJobState(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Update error passed through
}

func TestJobService_DeleteJob_Success(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.DeleteJobRequest{
		ID:     jobID,
		UserID: employerID, // User making request is employer
	}

	existingJob := &models.Job{
		ID:           jobID,
		EmployerID:   employerID,
		State:        models.JobStateWaiting, // Correct state
		ContractorID: nil,                   // No contractor
	}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	// Mock Delete call
	deleteReq := &dto.DeleteJobRequest{ID: jobID} // Service creates this internally
	mockJobRepo.EXPECT().Delete(ctx, deleteReq).Return(nil).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.NoError(t, err)
}

func TestJobService_DeleteJob_NotFound_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: uuid.New()}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, storage.ErrNotFound).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_DeleteJob_Forbidden_WrongUser(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	wrongUserID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: wrongUserID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrForbidden))
}

func TestJobService_DeleteJob_InvalidState_NotWaiting(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: employerID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateOngoing} // Wrong state

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidState))
}

func TestJobService_DeleteJob_InvalidState_ContractorAssigned(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	contractorID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: employerID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting, ContractorID: &contractorID} // Contractor assigned

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrInvalidState))
}

func TestJobService_DeleteJob_NotFound_Delete(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: employerID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	deleteReq := &dto.DeleteJobRequest{ID: jobID}
	mockJobRepo.EXPECT().Delete(ctx, deleteReq).Return(storage.ErrNotFound).Times(1) // Delete returns NotFound

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound)) // Service maps this
}

func TestJobService_DeleteJob_RepoError_GetByID(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: uuid.New()}
	repoErr := errors.New("db read failed")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(nil, repoErr).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal error fetching job for deletion")
	assert.True(t, errors.Is(err, repoErr))
}

func TestJobService_DeleteJob_RepoError_Delete(t *testing.T) {
	ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
	defer ctrl.Finish()

	jobID := uuid.New()
	employerID := uuid.New()
	req := &dto.DeleteJobRequest{ID: jobID, UserID: employerID}
	existingJob := &models.Job{ID: jobID, EmployerID: employerID, State: models.JobStateWaiting}
	repoErr := errors.New("db delete constraint")

	mockJobRepo.EXPECT().GetByID(ctx, &dto.GetJobByIDRequest{ID: jobID}).Return(existingJob, nil).Times(1)

	deleteReq := &dto.DeleteJobRequest{ID: jobID}
	mockJobRepo.EXPECT().Delete(ctx, deleteReq).Return(repoErr).Times(1)

	err := jobService.DeleteJob(ctx, req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repoErr)) // Delete error passed through
}