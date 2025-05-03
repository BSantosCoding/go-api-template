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

func TestJobService_CreateJob(t *testing.T) {
	type mockJobRepoCreate struct {
		req *dto.CreateJobRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name              string
		req               *dto.CreateJobRequest
		mockJobRepoCreate mockJobRepoCreate
		expectedJob       *models.Job
		expectedErr       error
		assertJob         func(*testing.T, *models.Job, *models.Job) // Custom assertion for job
	}{
		{
			name: "Success",
			req: &dto.CreateJobRequest{
				Rate:            100.50,
				Duration:        40,
				InvoiceInterval: 10,
				EmployerID:      uuid.New(),
			},
			mockJobRepoCreate: mockJobRepoCreate{
				req: &dto.CreateJobRequest{
					Rate:            100.50,
					Duration:        40,
					InvoiceInterval: 10,
					EmployerID:      uuid.Nil, 
				},
				res: &models.Job{
					ID:              uuid.New(), // Simulate repo generating ID
					Rate:            100.50,
					Duration:        40,
					InvoiceInterval: 10,
					EmployerID:      uuid.Nil, 
					State:           models.JobStateWaiting,
					CreatedAt:       time.Now(), // Simulate repo setting time
					UpdatedAt:       time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:              uuid.Nil, 
				Rate:            100.50,
				Duration:        40,
				InvoiceInterval: 10,
				EmployerID:      uuid.Nil, 
				State:           models.JobStateWaiting,
				CreatedAt:       time.Now(), // Will be asserted loosely
				UpdatedAt:       time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.NotEqual(t, uuid.Nil, actual.ID) // Ensure ID was generated
				assert.Equal(t, expected.Rate, actual.Rate)
				assert.Equal(t, expected.Duration, actual.Duration)
				assert.Equal(t, expected.InvoiceInterval, actual.InvoiceInterval)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				// Loosely assert time fields
				assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "RepoError",
			req: &dto.CreateJobRequest{
				Rate:            100.50,
				Duration:        40,
				InvoiceInterval: 10,
				EmployerID:      uuid.New(),
			},
			mockJobRepoCreate: mockJobRepoCreate{
				req: &dto.CreateJobRequest{
					Rate:            100.50,
					Duration:        40,
					InvoiceInterval: 10,
					EmployerID:      uuid.Nil, 
				},
				res: nil,
				err: errors.New("db connection failed"),
			},
			expectedJob: nil,
			expectedErr: errors.New("internal error creating job: db connection failed"), // Service wraps the error
			assertJob:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			employerID := uuid.New()
			tt.req.EmployerID = employerID
			tt.mockJobRepoCreate.req.EmployerID = employerID
			if tt.mockJobRepoCreate.res != nil {
				tt.mockJobRepoCreate.res.EmployerID = employerID
				// Simulate repo setting times if not already set
				if tt.mockJobRepoCreate.res.CreatedAt.IsZero() {
					tt.mockJobRepoCreate.res.CreatedAt = time.Now()
				}
				if tt.mockJobRepoCreate.res.UpdatedAt.IsZero() {
					tt.mockJobRepoCreate.res.UpdatedAt = time.Now()
				}
			}
			if tt.expectedJob != nil {
				tt.expectedJob.EmployerID = employerID
				// Simulate repo setting times if not already set
				if tt.expectedJob.CreatedAt.IsZero() {
					tt.expectedJob.CreatedAt = time.Now()
				}
				if tt.expectedJob.UpdatedAt.IsZero() {
					tt.expectedJob.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			mockJobRepo.EXPECT().Create(ctx, tt.mockJobRepoCreate.req).Return(tt.mockJobRepoCreate.res, tt.mockJobRepoCreate.err).Times(1)

			// Call the service method
			job, err := jobService.CreateJob(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error()) // Compare error strings for wrapped errors
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				if tt.assertJob != nil {
					tt.assertJob(t, tt.expectedJob, job)
				} else {
					assert.Equal(t, tt.expectedJob, job)
				}
			}
		})
	}
}

func TestJobService_GetJobByID(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name               string
		req                *dto.GetJobByIDRequest
		mockJobRepoGetByID mockJobRepoGetByID
		expectedJob        *models.Job
		expectedErr        error
	}{
		{
			name: "Success",
			req:  &dto.GetJobByIDRequest{ID: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), State: models.JobStateWaiting}, 
				err: nil,
			},
			expectedJob: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), State: models.JobStateWaiting}, 
			expectedErr: nil,
		},
		{
			name: "NotFound",
			req:  &dto.GetJobByIDRequest{ID: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			expectedJob: nil,
			expectedErr: services.ErrNotFound,
		},
		{
			name: "RepoError",
			req:  &dto.GetJobByIDRequest{ID: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read error"),
			},
			expectedJob: nil,
			expectedErr: errors.New("internal error getting job: db read error"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			tt.req.ID = jobID
			tt.mockJobRepoGetByID.req.ID = jobID
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				tt.mockJobRepoGetByID.res.EmployerID = employerID
			}
			if tt.expectedJob != nil {
				tt.expectedJob.ID = jobID
				tt.expectedJob.EmployerID = employerID
			}

			// Setup mocks
			mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)

			// Call the service method
			job, err := jobService.GetJobByID(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				assert.Equal(t, tt.expectedJob, job)
			}
		})
	}
}

func TestJobService_ListAvailableJobs(t *testing.T) {
	type mockJobRepoListAvailable struct {
		req *dto.ListAvailableJobsRequest
		res []models.Job
		err error
	}

	tests := []struct {
		name                     string
		req                      *dto.ListAvailableJobsRequest
		mockJobRepoListAvailable mockJobRepoListAvailable
		expectedJobs             []models.Job
		expectedErr              error
	}{
		{
			name: "Success",
			req:  &dto.ListAvailableJobsRequest{Limit: 5, Offset: 0},
			mockJobRepoListAvailable: mockJobRepoListAvailable{
				req: &dto.ListAvailableJobsRequest{Limit: 5, Offset: 0},
				res: []models.Job{
					{ID: uuid.New(), State: models.JobStateWaiting, EmployerID: uuid.New()},
					{ID: uuid.New(), State: models.JobStateWaiting, EmployerID: uuid.New()},
				},
				err: nil,
			},
			expectedJobs: []models.Job{
				{ID: uuid.Nil, State: models.JobStateWaiting, EmployerID: uuid.New()},
				{ID: uuid.Nil, State: models.JobStateWaiting, EmployerID: uuid.New()}, 
			},
			expectedErr: nil,
		},
		{
			name: "RepoError",
			req:  &dto.ListAvailableJobsRequest{Limit: 10, Offset: 0},
			mockJobRepoListAvailable: mockJobRepoListAvailable{
				req: &dto.ListAvailableJobsRequest{Limit: 10, Offset: 0},
				res: nil,
				err: errors.New("db query failed"),
			},
			expectedJobs: nil,
			expectedErr:  errors.New("internal error listing available jobs: db query failed"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs for expected jobs
			for i := range tt.expectedJobs {
				tt.expectedJobs[i].ID = uuid.New()
				tt.expectedJobs[i].EmployerID = uuid.New()
			}
			// Set dynamic UUIDs for mock response jobs
			for i := range tt.mockJobRepoListAvailable.res {
				tt.mockJobRepoListAvailable.res[i].ID = tt.expectedJobs[i].ID // Match expected IDs
				tt.mockJobRepoListAvailable.res[i].EmployerID = tt.expectedJobs[i].EmployerID
			}

			// Setup mocks
			mockJobRepo.EXPECT().ListAvailable(ctx, tt.req).Return(tt.mockJobRepoListAvailable.res, tt.mockJobRepoListAvailable.err).Times(1)

			// Call the service method
			jobs, err := jobService.ListAvailableJobs(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error()) // Compare error strings for wrapped errors
				assert.Nil(t, jobs)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedJobs, jobs)
			}
		})
	}
}

func TestJobService_ListJobsByEmployer(t *testing.T) {
	type mockJobRepoListByEmployer struct {
		req *dto.ListJobsByEmployerRequest
		res []models.Job
		err error
	}

	tests := []struct {
		name                      string
		req                       *dto.ListJobsByEmployerRequest
		mockJobRepoListByEmployer mockJobRepoListByEmployer
		expectedJobs              []models.Job
		expectedErr               error
	}{
		{
			name: "Success",
			req:  &dto.ListJobsByEmployerRequest{EmployerID: uuid.New(), Limit: 5, Offset: 0},
			mockJobRepoListByEmployer: mockJobRepoListByEmployer{
				req: &dto.ListJobsByEmployerRequest{EmployerID: uuid.Nil, Limit: 5, Offset: 0}, 
				res: []models.Job{
					{ID: uuid.New(), EmployerID: uuid.Nil, State: models.JobStateWaiting}, 
					{ID: uuid.New(), EmployerID: uuid.Nil, State: models.JobStateOngoing, ContractorID: ptrUUID(uuid.New())}, 
				},
				err: nil,
			},
			expectedJobs: []models.Job{
				{ID: uuid.Nil, EmployerID: uuid.Nil, State: models.JobStateWaiting}, 
				{ID: uuid.Nil, EmployerID: uuid.Nil, State: models.JobStateOngoing, ContractorID: ptrUUID(uuid.New())}, 
			},
			expectedErr: nil,
		},
		{
			name: "RepoError",
			req:  &dto.ListJobsByEmployerRequest{EmployerID: uuid.New(), Limit: 10, Offset: 0},
			mockJobRepoListByEmployer: mockJobRepoListByEmployer{
				req: &dto.ListJobsByEmployerRequest{EmployerID: uuid.Nil, Limit: 10, Offset: 0}, 
				res: nil,
				err: errors.New("db query failed"),
			},
			expectedJobs: nil,
			expectedErr:  errors.New("internal error listing employer jobs: db query failed"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			employerID := uuid.New()
			tt.req.EmployerID = employerID
			tt.mockJobRepoListByEmployer.req.EmployerID = employerID
			for i := range tt.mockJobRepoListByEmployer.res {
				tt.mockJobRepoListByEmployer.res[i].EmployerID = employerID
				if tt.mockJobRepoListByEmployer.res[i].ContractorID != nil {
					*tt.mockJobRepoListByEmployer.res[i].ContractorID = uuid.New() // Assign a new UUID for contractor
				}
				tt.mockJobRepoListByEmployer.res[i].ID = uuid.New() // Assign new UUID for job ID
			}
			for i := range tt.expectedJobs {
				tt.expectedJobs[i].EmployerID = employerID
				if tt.expectedJobs[i].ContractorID != nil {
					*tt.expectedJobs[i].ContractorID = uuid.New() // Assign a new UUID for contractor
				}
				tt.expectedJobs[i].ID = tt.mockJobRepoListByEmployer.res[i].ID // Match mock response IDs
				if tt.expectedJobs[i].ContractorID != nil && tt.mockJobRepoListByEmployer.res[i].ContractorID != nil {
					*tt.expectedJobs[i].ContractorID = *tt.mockJobRepoListByEmployer.res[i].ContractorID // Match contractor IDs
				}
			}

			// Setup mocks
			mockJobRepo.EXPECT().ListByEmployer(ctx, tt.mockJobRepoListByEmployer.req).Return(tt.mockJobRepoListByEmployer.res, tt.mockJobRepoListByEmployer.err).Times(1)

			// Call the service method
			jobs, err := jobService.ListJobsByEmployer(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error()) // Compare error strings for wrapped errors
				assert.Nil(t, jobs)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedJobs, jobs)
			}
		})
	}
}

func TestJobService_ListJobsByContractor(t *testing.T) {
	type mockJobRepoListByContractor struct {
		req *dto.ListJobsByContractorRequest
		res []models.Job
		err error
	}

	tests := []struct {
		name                        string
		req                         *dto.ListJobsByContractorRequest
		mockJobRepoListByContractor mockJobRepoListByContractor
		expectedJobs                []models.Job
		expectedErr                 error
	}{
		{
			name: "Success",
			req:  &dto.ListJobsByContractorRequest{ContractorID: uuid.New(), Limit: 5, Offset: 0},
			mockJobRepoListByContractor: mockJobRepoListByContractor{
				req: &dto.ListJobsByContractorRequest{ContractorID: uuid.Nil, Limit: 5, Offset: 0}, 
				res: []models.Job{
					{ID: uuid.New(), EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: ptrUUID(uuid.Nil)}, 
					{ID: uuid.New(), EmployerID: uuid.New(), State: models.JobStateComplete, ContractorID: ptrUUID(uuid.Nil)}, 
				},
				err: nil,
			},
			expectedJobs: []models.Job{
				{ID: uuid.Nil, EmployerID: uuid.New(), State: models.JobStateOngoing, ContractorID: ptrUUID(uuid.Nil)}, 
				{ID: uuid.Nil, EmployerID: uuid.New(), State: models.JobStateComplete, ContractorID: ptrUUID(uuid.Nil)}, 
			},
			expectedErr: nil,
		},
		{
			name: "RepoError",
			req:  &dto.ListJobsByContractorRequest{ContractorID: uuid.New(), Limit: 10, Offset: 0},
			mockJobRepoListByContractor: mockJobRepoListByContractor{
				req: &dto.ListJobsByContractorRequest{ContractorID: uuid.Nil, Limit: 10, Offset: 0}, 
				res: nil,
				err: errors.New("db query failed"),
			},
			expectedJobs: nil,
			expectedErr:  errors.New("internal error listing contractor jobs: db query failed"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			contractorID := uuid.New()
			tt.req.ContractorID = contractorID
			tt.mockJobRepoListByContractor.req.ContractorID = contractorID
			for i := range tt.mockJobRepoListByContractor.res {
				tt.mockJobRepoListByContractor.res[i].ContractorID = &contractorID
				tt.mockJobRepoListByContractor.res[i].ID = uuid.New() // Assign new UUID for job ID
				tt.mockJobRepoListByContractor.res[i].EmployerID = uuid.New() // Assign new UUID for employer ID
			}
			for i := range tt.expectedJobs {
				tt.expectedJobs[i].ContractorID = &contractorID
				tt.expectedJobs[i].ID = tt.mockJobRepoListByContractor.res[i].ID // Match mock response IDs
				tt.expectedJobs[i].EmployerID = tt.mockJobRepoListByContractor.res[i].EmployerID // Match employer IDs
			}

			// Setup mocks
			mockJobRepo.EXPECT().ListByContractor(ctx, tt.mockJobRepoListByContractor.req).Return(tt.mockJobRepoListByContractor.res, tt.mockJobRepoListByContractor.err).Times(1)

			// Call the service method
			jobs, err := jobService.ListJobsByContractor(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error()) // Compare error strings for wrapped errors
				assert.Nil(t, jobs)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedJobs, jobs)
			}
		})
	}
}

func TestJobService_UpdateJobDetails(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockJobRepoUpdate struct {
		req *dto.UpdateJobRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name              string
		req               *dto.UpdateJobDetailsRequest
		mockJobRepoGetByID mockJobRepoGetByID
		mockJobRepoUpdate mockJobRepoUpdate
		expectedJob       *models.Job
		expectedErr       error
		assertJob         func(*testing.T, *models.Job, *models.Job) // Custom assertion for job
	}{
		{
			name: "Success",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(),
				Rate:     ptrFloat64(120.0),
				Duration: ptrInt(50),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.Nil, 
					State:      models.JobStateWaiting,
					ContractorID: nil,
					Rate:       100.0,
					Duration:   40,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:       uuid.Nil, 
					Rate:     ptrFloat64(120.0),
					Duration: ptrInt(50),
				},
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.Nil, 
					State:      models.JobStateWaiting,
					ContractorID: nil,
					Rate:       120.0,
					Duration:   50,
					UpdatedAt:  time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:         uuid.Nil, 
				EmployerID: uuid.Nil, 
				State:      models.JobStateWaiting,
				ContractorID: nil,
				Rate:       120.0,
				Duration:   50,
				UpdatedAt:  time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.Equal(t, expected.Rate, actual.Rate)
				assert.Equal(t, expected.Duration, actual.Duration)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "NotFound",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(),
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrNotFound,
			assertJob:         nil,
		},
		{
			name: "Forbidden_WrongUser",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(), // Wrong user
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.New(), // Actual employer
					State:      models.JobStateWaiting,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "Forbidden_WrongState",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(), // Employer
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.Nil, 
					State:      models.JobStateOngoing, // Wrong state
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden, // Service maps this specific check to Forbidden
			assertJob:         nil,
		},
		{
			name: "Forbidden_ContractorAssigned",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(), // Employer
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.Nil, 
					State:      models.JobStateWaiting,
					ContractorID: ptrUUID(uuid.New()), // Contractor assigned
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden, // Service maps this specific check to Forbidden
			assertJob:         nil,
		},
		{
			name: "RepoError_GetByID",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(),
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read failed"),
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       errors.New("internal error fetching job for update: db read failed"), // Service wraps the error
			assertJob:         nil,
		},
		{
			name: "RepoError_Update",
			req: &dto.UpdateJobDetailsRequest{
				JobID:    uuid.New(),
				UserID:   uuid.New(), // Employer
				Rate:     ptrFloat64(110.0),
				Duration: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:         uuid.Nil, 
					EmployerID: uuid.Nil, 
					State:      models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:       uuid.Nil, 
					Rate:     ptrFloat64(110.0),
					Duration: nil,
				},
				res: nil,
				err: errors.New("db write failed"),
			},
			expectedJob: nil,
			expectedErr: errors.New("db write failed"), // Update error is passed through
			assertJob:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			contractorID := uuid.New()
			wrongUserID := uuid.New()

			tt.req.JobID = jobID
			if tt.name == "Success" || tt.name == "RepoError_Update" {
				tt.req.UserID = employerID
			} else if tt.name == "Forbidden_WrongUser" {
				tt.req.UserID = wrongUserID
			} else if tt.name == "Forbidden_WrongState" || tt.name == "Forbidden_ContractorAssigned" {
				tt.req.UserID = employerID // Assume employer for these cases
			} else {
				tt.req.UserID = uuid.New() // Default for other cases
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success" || tt.name == "RepoError_Update" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = nil
				} else if tt.name == "Forbidden_WrongUser" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
				} else if tt.name == "Forbidden_WrongState" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.State = models.JobStateOngoing
				} else if tt.name == "Forbidden_ContractorAssigned" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Default
				}
			}

			if tt.mockJobRepoUpdate.req != nil {
				tt.mockJobRepoUpdate.req.ID = jobID
			}
			if tt.mockJobRepoUpdate.res != nil {
				tt.mockJobRepoUpdate.res.ID = jobID
				tt.mockJobRepoUpdate.res.EmployerID = employerID
				// Simulate repo setting time if not already set
				if tt.mockJobRepoUpdate.res.UpdatedAt.IsZero() {
					tt.mockJobRepoUpdate.res.UpdatedAt = time.Now()
				}
			}

			if tt.expectedJob != nil {
				tt.expectedJob.ID = jobID
				tt.expectedJob.EmployerID = employerID
				// Simulate repo setting time if not already set
				if tt.expectedJob.UpdatedAt.IsZero() {
					tt.expectedJob.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoUpdate.req != nil || tt.mockJobRepoUpdate.err != nil {
				mockJobRepo.EXPECT().Update(ctx, gomock.Any()).Return(tt.mockJobRepoUpdate.res, tt.mockJobRepoUpdate.err).Times(1)
			}

			// Call the service method
			job, err := jobService.UpdateJobDetails(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				if tt.assertJob != nil {
					tt.assertJob(t, tt.expectedJob, job)
				} else {
					// Loosely assert time fields
					assert.WithinDuration(t, tt.expectedJob.UpdatedAt, job.UpdatedAt, time.Second)
					// Compare other fields
					tt.expectedJob.UpdatedAt = job.UpdatedAt // Set expected to actual for comparison
					assert.Equal(t, tt.expectedJob, job)
				}
			}
		})
	}
}

func TestJobService_AssignContractor(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockJobRepoUpdate struct {
		req *dto.UpdateJobRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name              string
		req               *dto.AssignContractorRequest
		mockJobRepoGetByID mockJobRepoGetByID
		mockJobRepoUpdate mockJobRepoUpdate
		expectedJob       *models.Job
		expectedErr       error
		assertJob         func(*testing.T, *models.Job, *models.Job) // Custom assertion for job
	}{
		{
			name: "Success_EmployerAssigns",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Employer making request
				ContractorID: uuid.New(), // Assigning another user
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: ptrUUID(uuid.Nil), 
					State:        ptrJobState(models.JobStateOngoing),
				},
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.Nil), 
					UpdatedAt:    time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:           uuid.Nil, 
				EmployerID:   uuid.Nil, 
				State:        models.JobStateOngoing,
				ContractorID: ptrUUID(uuid.Nil), 
				UpdatedAt:    time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "Success_ContractorAssignsSelf",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Contractor making request
				ContractorID: uuid.Nil, 
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: ptrUUID(uuid.Nil), 
					State:        ptrJobState(models.JobStateOngoing),
				},
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.Nil), 
					UpdatedAt:    time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:           uuid.Nil, 
				EmployerID:   uuid.Nil, 
				State:        models.JobStateOngoing,
				ContractorID: ptrUUID(uuid.Nil), 
				UpdatedAt:    time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "NotFound",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(),
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrNotFound,
			assertJob:         nil,
		},
		{
			name: "InvalidState_NotWaiting",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(),
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing, // Wrong state
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrInvalidState,
			assertJob:         nil,
		},
		{
			name: "InvalidState_ContractorExists",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(),
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateWaiting,
					ContractorID: ptrUUID(uuid.New()), // Contractor exists
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrInvalidState,
			assertJob:         nil,
		},
		{
			name: "Forbidden_EmployerAssignsSelf",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Employer
				ContractorID: uuid.Nil, 
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "Forbidden_NonEmployerAssignsOther",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Non-employer
				ContractorID: uuid.New(), // Assigning someone else
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(), // Actual employer
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "Conflict_Update",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Employer
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: ptrUUID(uuid.Nil), 
					State:        ptrJobState(models.JobStateOngoing),
				},
				res: nil,
				err: storage.ErrConflict, // Simulate FK violation etc.
			},
			expectedJob: nil,
			expectedErr: services.ErrConflict,
			assertJob:   nil,
		},
		{
			name: "RepoError_GetByID",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(),
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read failed"),
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       errors.New("internal error fetching job for assignment: db read failed"), // Service wraps the error
			assertJob:         nil,
		},
		{
			name: "RepoError_Update",
			req: &dto.AssignContractorRequest{
				JobID:        uuid.New(),
				UserID:       uuid.New(), // Employer
				ContractorID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: ptrUUID(uuid.Nil), 
					State:        ptrJobState(models.JobStateOngoing),
				},
				res: nil,
				err: errors.New("db write failed"),
			},
			expectedJob: nil,
			expectedErr: errors.New("db write failed"), // Update error passed through
			assertJob:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			contractorID := uuid.New()
			requestingUserID := uuid.New()
			targetContractorID := uuid.New()

			tt.req.JobID = jobID
			if tt.name == "Success_EmployerAssigns" || tt.name == "Conflict_Update" || tt.name == "RepoError_Update" {
				tt.req.UserID = employerID
				tt.req.ContractorID = contractorID
			} else if tt.name == "Success_ContractorAssignsSelf" {
				tt.req.UserID = contractorID
				tt.req.ContractorID = contractorID
			} else if tt.name == "Forbidden_EmployerAssignsSelf" {
				tt.req.UserID = employerID
				tt.req.ContractorID = employerID
			} else if tt.name == "Forbidden_NonEmployerAssignsOther" {
				tt.req.UserID = requestingUserID
				tt.req.ContractorID = targetContractorID
			} else {
				tt.req.UserID = uuid.New() // Default
				tt.req.ContractorID = uuid.New() // Default
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success_EmployerAssigns" || tt.name == "Success_ContractorAssignsSelf" || tt.name == "Forbidden_EmployerAssignsSelf" || tt.name == "Conflict_Update" || tt.name == "RepoError_Update" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
				} else if tt.name == "InvalidState_NotWaiting" {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New()
					tt.mockJobRepoGetByID.res.State = models.JobStateOngoing
				} else if tt.name == "InvalidState_ContractorExists" {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New()
					tt.mockJobRepoGetByID.res.ContractorID = ptrUUID(uuid.New())
				} else if tt.name == "Forbidden_NonEmployerAssignsOther" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
				} else {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Default
				}
			}

			if tt.mockJobRepoUpdate.req != nil {
				tt.mockJobRepoUpdate.req.ID = jobID
				if tt.mockJobRepoUpdate.req.ContractorID != nil {
					*tt.mockJobRepoUpdate.req.ContractorID = tt.req.ContractorID // Match request contractor ID
				}
			}
			if tt.mockJobRepoUpdate.res != nil {
				tt.mockJobRepoUpdate.res.ID = jobID
				tt.mockJobRepoUpdate.res.EmployerID = employerID
				if tt.mockJobRepoUpdate.res.ContractorID != nil {
					*tt.mockJobRepoUpdate.res.ContractorID = tt.req.ContractorID // Match request contractor ID
				}
				// Simulate repo setting time if not already set
				if tt.mockJobRepoUpdate.res.UpdatedAt.IsZero() {
					tt.mockJobRepoUpdate.res.UpdatedAt = time.Now()
				}
			}

			if tt.expectedJob != nil {
				tt.expectedJob.ID = jobID
				tt.expectedJob.EmployerID = employerID
				if tt.expectedJob.ContractorID != nil {
					*tt.expectedJob.ContractorID = tt.req.ContractorID // Match request contractor ID
				}
				// Simulate repo setting time if not already set
				if tt.expectedJob.UpdatedAt.IsZero() {
					tt.expectedJob.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoUpdate.req != nil || tt.mockJobRepoUpdate.err != nil {
				mockJobRepo.EXPECT().Update(ctx, gomock.Any()).Return(tt.mockJobRepoUpdate.res, tt.mockJobRepoUpdate.err).Times(1)
			}

			// Call the service method
			job, err := jobService.AssignContractor(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				if tt.assertJob != nil {
					tt.assertJob(t, tt.expectedJob, job)
				} else {
					// Loosely assert time fields
					assert.WithinDuration(t, tt.expectedJob.UpdatedAt, job.UpdatedAt, time.Second)
					// Compare other fields
					tt.expectedJob.UpdatedAt = job.UpdatedAt // Set expected to actual for comparison
					assert.Equal(t, tt.expectedJob, job)
				}
			}
		})
	}
}

func TestJobService_UnassignContractor(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockJobRepoUpdate struct {
		req *dto.UpdateJobRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name              string
		req               *dto.UnassignContractorRequest
		mockJobRepoGetByID mockJobRepoGetByID
		mockJobRepoUpdate mockJobRepoUpdate
		expectedJob       *models.Job
		expectedErr       error
		assertJob         func(*testing.T, *models.Job, *models.Job) // Custom assertion for job
	}{
		{
			name: "Success",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Current contractor
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.Nil), 
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: nil,
					State:        ptrJobState(models.JobStateWaiting),
				},
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
					UpdatedAt:    time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:           uuid.Nil, 
				EmployerID:   uuid.Nil, 
				State:        models.JobStateWaiting,
				ContractorID: nil,
				UpdatedAt:    time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "NotFound",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrNotFound,
			assertJob:         nil,
		},
		{
			name: "Forbidden_WrongUser",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Wrong user
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.New()), // Actual contractor
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "Forbidden_NoContractor",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Doesn't matter who tries
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing,
					ContractorID: nil, // No contractor
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "Forbidden_WrongState",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Current contractor
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateWaiting, // Wrong state
					ContractorID: ptrUUID(uuid.Nil), 
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "RepoError_GetByID",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read failed"),
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       errors.New("internal error fetching job for unassignment: db read failed"), // Service wraps the error
			assertJob:         nil,
		},
		{
			name: "RepoError_Update",
			req: &dto.UnassignContractorRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Current contractor
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.Nil), 
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:           uuid.Nil, 
					ContractorID: nil,
					State:        ptrJobState(models.JobStateWaiting),
				},
				res: nil,
				err: errors.New("db write failed"),
			},
			expectedJob: nil,
			expectedErr: errors.New("db write failed"), // Update error passed through
			assertJob:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			contractorID := uuid.New()
			wrongUserID := uuid.New()
			employerID := uuid.New()

			tt.req.JobID = jobID
			if tt.name == "Success" || tt.name == "Forbidden_WrongState" || tt.name == "RepoError_Update" {
				tt.req.UserID = contractorID
			} else if tt.name == "Forbidden_WrongUser" {
				tt.req.UserID = wrongUserID
			} else {
				tt.req.UserID = uuid.New() // Default
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Assign new employer ID
				if tt.name == "Success" || tt.name == "Forbidden_WrongUser" || tt.name == "RepoError_Update" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Forbidden_WrongState" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
					tt.mockJobRepoGetByID.res.State = models.JobStateWaiting
				} else {
					tt.mockJobRepoGetByID.res.ContractorID = nil // Default
				}
			}

			if tt.mockJobRepoUpdate.req != nil {
				tt.mockJobRepoUpdate.req.ID = jobID
			}
			if tt.mockJobRepoUpdate.res != nil {
				tt.mockJobRepoUpdate.res.ID = jobID
				tt.mockJobRepoUpdate.res.EmployerID = employerID // Assign new employer ID
				// Simulate repo setting time if not already set
				if tt.mockJobRepoUpdate.res.UpdatedAt.IsZero() {
					tt.mockJobRepoUpdate.res.UpdatedAt = time.Now()
				}
			}

			if tt.expectedJob != nil {
				tt.expectedJob.ID = jobID
				tt.expectedJob.EmployerID = employerID // Assign new employer ID
				// Simulate repo setting time if not already set
				if tt.expectedJob.UpdatedAt.IsZero() {
					tt.expectedJob.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoUpdate.req != nil || tt.mockJobRepoUpdate.err != nil {
				mockJobRepo.EXPECT().Update(ctx, gomock.Any()).Return(tt.mockJobRepoUpdate.res, tt.mockJobRepoUpdate.err).Times(1)
			}

			// Call the service method
			job, err := jobService.UnassignContractor(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				if tt.assertJob != nil {
					tt.assertJob(t, tt.expectedJob, job)
				} else {
					// Loosely assert time fields
					assert.WithinDuration(t, tt.expectedJob.UpdatedAt, job.UpdatedAt, time.Second)
					// Compare other fields
					tt.expectedJob.UpdatedAt = job.UpdatedAt // Set expected to actual for comparison
					assert.Equal(t, tt.expectedJob, job)
				}
			}
		})
	}
}

func TestJobService_UpdateJobState(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockJobRepoUpdate struct {
		req *dto.UpdateJobRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name              string
		req               *dto.UpdateJobStateRequest
		mockJobRepoGetByID mockJobRepoGetByID
		mockJobRepoUpdate mockJobRepoUpdate
		expectedJob       *models.Job
		expectedErr       error
		assertJob         func(*testing.T, *models.Job, *models.Job) // Custom assertion for job
	}{
		{
			name: "Success_Employer",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Employer
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.New()),
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:    uuid.Nil, 
					State: ptrJobState(models.JobStateComplete),
				},
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateComplete,
					ContractorID: ptrUUID(uuid.New()),
					UpdatedAt:    time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:           uuid.Nil, 
				EmployerID:   uuid.Nil, 
				State:        models.JobStateComplete,
				ContractorID: ptrUUID(uuid.New()),
				UpdatedAt:    time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "Success_Contractor",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Contractor
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.Nil), 
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:    uuid.Nil, 
					State: ptrJobState(models.JobStateComplete),
				},
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateComplete,
					ContractorID: ptrUUID(uuid.Nil), 
					UpdatedAt:    time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedJob: &models.Job{
				ID:           uuid.Nil, 
				EmployerID:   uuid.Nil, 
				State:        models.JobStateComplete,
				ContractorID: ptrUUID(uuid.Nil), 
				UpdatedAt:    time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertJob: func(t *testing.T, expected, actual *models.Job) {
				assert.Equal(t, expected.ID, actual.ID)
				assert.Equal(t, expected.EmployerID, actual.EmployerID)
				assert.Equal(t, expected.State, actual.State)
				assert.Equal(t, expected.ContractorID, actual.ContractorID)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "NotFound",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(),
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrNotFound,
			assertJob:         nil,
		},
		{
			name: "Forbidden_WrongUser",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Wrong user
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(), // Actual employer
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.New()), // Actual contractor
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrForbidden,
			assertJob:         nil,
		},
		{
			name: "InvalidTransition",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Employer
				State:  models.JobStateWaiting, // Invalid: Complete -> Waiting
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateComplete,
					ContractorID: ptrUUID(uuid.New()),
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       services.ErrInvalidTransition,
			assertJob:         nil,
		},
		{
			name: "RepoError_GetByID",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(),
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read failed"),
			},
			mockJobRepoUpdate: mockJobRepoUpdate{}, // Not called
			expectedJob:       nil,
			expectedErr:       errors.New("internal error fetching job for state update: db read failed"), // Service wraps the error
			assertJob:         nil,
		},
		{
			name: "RepoError_Update",
			req: &dto.UpdateJobStateRequest{
				JobID:  uuid.New(),
				UserID: uuid.New(), // Employer
				State:  models.JobStateComplete,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateOngoing,
					ContractorID: ptrUUID(uuid.New()),
				},
				err: nil,
			},
			mockJobRepoUpdate: mockJobRepoUpdate{
				req: &dto.UpdateJobRequest{
					ID:    uuid.Nil, 
					State: ptrJobState(models.JobStateComplete),
				},
				res: nil,
				err: errors.New("db write failed"),
			},
			expectedJob: nil,
			expectedErr: errors.New("db write failed"), // Update error passed through
			assertJob:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			contractorID := uuid.New()
			wrongUserID := uuid.New()

			tt.req.JobID = jobID
			if tt.name == "Success_Employer" || tt.name == "InvalidTransition" || tt.name == "RepoError_Update" {
				tt.req.UserID = employerID
			} else if tt.name == "Success_Contractor" {
				tt.req.UserID = contractorID
			} else if tt.name == "Forbidden_WrongUser" {
				tt.req.UserID = wrongUserID
			} else {
				tt.req.UserID = uuid.New() // Default
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success_Employer" || tt.name == "InvalidTransition" || tt.name == "RepoError_Update" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Success_Contractor" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Forbidden_WrongUser" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Default
					tt.mockJobRepoGetByID.res.ContractorID = ptrUUID(uuid.New()) // Default
				}
			}

			if tt.mockJobRepoUpdate.req != nil {
				tt.mockJobRepoUpdate.req.ID = jobID
			}
			if tt.mockJobRepoUpdate.res != nil {
				tt.mockJobRepoUpdate.res.ID = jobID
				if tt.name == "Success_Employer" || tt.name == "Success_Contractor" || tt.name == "RepoError_Update" {
					tt.mockJobRepoUpdate.res.EmployerID = employerID
					tt.mockJobRepoUpdate.res.ContractorID = &contractorID
				} else {
					tt.mockJobRepoUpdate.res.EmployerID = uuid.New() // Default
					tt.mockJobRepoUpdate.res.ContractorID = ptrUUID(uuid.New()) // Default
				}
				// Simulate repo setting time if not already set
				if tt.mockJobRepoUpdate.res.UpdatedAt.IsZero() {
					tt.mockJobRepoUpdate.res.UpdatedAt = time.Now()
				}
			}

			if tt.expectedJob != nil {
				tt.expectedJob.ID = jobID
				if tt.name == "Success_Employer" || tt.name == "Success_Contractor" {
					tt.expectedJob.EmployerID = employerID
					tt.expectedJob.ContractorID = &contractorID
				} else {
					tt.expectedJob.EmployerID = uuid.New() // Default
					tt.expectedJob.ContractorID = ptrUUID(uuid.New()) // Default
				}
				// Simulate repo setting time if not already set
				if tt.expectedJob.UpdatedAt.IsZero() {
					tt.expectedJob.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoUpdate.req != nil || tt.mockJobRepoUpdate.err != nil {
				mockJobRepo.EXPECT().Update(ctx, gomock.Any()).Return(tt.mockJobRepoUpdate.res, tt.mockJobRepoUpdate.err).Times(1)
			}

			// Call the service method
			job, err := jobService.UpdateJobState(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, job)
				if tt.assertJob != nil {
					tt.assertJob(t, tt.expectedJob, job)
				} else {
					// Loosely assert time fields
					assert.WithinDuration(t, tt.expectedJob.UpdatedAt, job.UpdatedAt, time.Second)
					// Compare other fields
					tt.expectedJob.UpdatedAt = job.UpdatedAt // Set expected to actual for comparison
					assert.Equal(t, tt.expectedJob, job)
				}
			}
		})
	}
}

func TestJobService_DeleteJob(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockJobRepoDelete struct {
		req *dto.DeleteJobRequest
		err error
	}

	tests := []struct {
		name              string
		req               *dto.DeleteJobRequest
		mockJobRepoGetByID mockJobRepoGetByID
		mockJobRepoDelete mockJobRepoDelete
		expectedErr       error
	}{
		{
			name: "Success",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Employer
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{
				req: &dto.DeleteJobRequest{ID: uuid.Nil}, 
				err: nil,
			},
			expectedErr: nil,
		},
		{
			name: "NotFound_GetByID",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoDelete: mockJobRepoDelete{}, // Not called
			expectedErr: services.ErrNotFound,
		},
		{
			name: "Forbidden_WrongUser",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Wrong user
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(), // Actual employer
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{}, // Not called
			expectedErr: services.ErrForbidden,
		},
		{
			name: "InvalidState_NotWaiting",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Employer
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateOngoing, // Wrong state
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{}, // Not called
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "InvalidState_ContractorAssigned",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Employer
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: ptrUUID(uuid.New()), // Contractor assigned
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{}, // Not called
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "NotFound_Delete",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Employer
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{
				req: &dto.DeleteJobRequest{ID: uuid.Nil}, 
				err: storage.ErrNotFound, // Delete returns NotFound
			},
			expectedErr: services.ErrNotFound, // Service maps this
		},
		{
			name: "RepoError_GetByID",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db read failed"),
			},
			mockJobRepoDelete: mockJobRepoDelete{}, // Not called
			expectedErr: errors.New("internal error fetching job for deletion: db read failed"), // Service wraps the error
		},
		{
			name: "RepoError_Delete",
			req: &dto.DeleteJobRequest{
				ID:     uuid.New(),
				UserID: uuid.New(), // Employer
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.Nil, 
					State:        models.JobStateWaiting,
					ContractorID: nil,
				},
				err: nil,
			},
			mockJobRepoDelete: mockJobRepoDelete{
				req: &dto.DeleteJobRequest{ID: uuid.Nil}, 
				err: errors.New("db delete constraint"),
			},
			expectedErr: errors.New("db delete constraint"), // Delete error passed through
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, jobService, mockJobRepo, _, ctrl := setupJobServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			wrongUserID := uuid.New()
			contractorID := uuid.New()

			tt.req.ID = jobID
			if tt.name == "Success" || tt.name == "InvalidState_NotWaiting" || tt.name == "InvalidState_ContractorAssigned" || tt.name == "NotFound_Delete" || tt.name == "RepoError_Delete" {
				tt.req.UserID = employerID
			} else if tt.name == "Forbidden_WrongUser" {
				tt.req.UserID = wrongUserID
			} else {
				tt.req.UserID = uuid.New() // Default
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success" || tt.name == "NotFound_Delete" || tt.name == "RepoError_Delete" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = nil
				} else if tt.name == "Forbidden_WrongUser" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
				} else if tt.name == "InvalidState_NotWaiting" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.State = models.JobStateOngoing
				} else if tt.name == "InvalidState_ContractorAssigned" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Default
				}
			}

			if tt.mockJobRepoDelete.req != nil {
				tt.mockJobRepoDelete.req.ID = jobID
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoDelete.req != nil || tt.mockJobRepoDelete.err != nil {
				mockJobRepo.EXPECT().Delete(ctx, gomock.Any()).Return(tt.mockJobRepoDelete.err).Times(1)
			}

			// Call the service method
			err := jobService.DeleteJob(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
