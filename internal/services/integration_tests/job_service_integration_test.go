package integration_tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-api-template/ent"
	"go-api-template/ent/job"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"          // For storage errors
	"go-api-template/internal/storage/postgres" // Need concrete repo for setup/assertion
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a pointer to a JobState
func ptrJobState(s job.State) *job.State { return &s }

// setupJobServiceIntegrationTest initializes the service with a real DB pool.
func setupJobServiceIntegrationTest(t *testing.T) (context.Context, services.JobService, *ent.Client) {
	t.Helper() // Mark as test helper
	pool, _ := getTestClients(t)
	// Instantiate the real service using the constructor that creates repos internally
	jobService := services.NewJobService(pool)
	ctx := context.Background()
	return ctx, jobService, pool
}

func TestJobService_Integration_CreateJobAndGetByID(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs")
	jobRepo := postgres.NewJobRepo(pool) // Need for verification
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	// Create prerequisite employer
	employer := createTestUser(t, ctx, pool, "createjob-employer@test.com", "CreateJob Employer")

	// --- Create Job ---
	createReq := &dto.CreateJobRequest{
		Rate:            150.75,
		Duration:        60,
		InvoiceInterval: 15,
		EmployerID:      employer.ID, // Set by handler in real scenario, set here for test
	}
	createdJob, err := jobService.CreateJob(ctx, createReq)

	// --- Assert Create ---
	require.NoError(t, err)
	require.NotNil(t, createdJob)
	assert.NotEqual(t, uuid.Nil, createdJob.ID)
	assert.Equal(t, createReq.Rate, createdJob.Rate)
	assert.Equal(t, createReq.Duration, createdJob.Duration)
	assert.Equal(t, createReq.InvoiceInterval, createdJob.InvoiceInterval)
	assert.Equal(t, createReq.EmployerID, createdJob.EmployerID)
	assert.Equal(t, job.StateWaiting, createdJob.State)
	assert.Equal(t, uuid.Nil, createdJob.ContractorID)

	// Verify directly in DB
	dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: createdJob.ID})
	require.NoError(t, dbErr)
	require.NotNil(t, dbJob)
	assert.Equal(t, createdJob.ID, dbJob.ID)
	assert.Equal(t, createReq.Rate, dbJob.Rate)
	assert.Equal(t, job.StateWaiting, dbJob.State)

	// --- Get Job By ID ---
	getReq := &dto.GetJobByIDRequest{ID: createdJob.ID}
	fetchedJob, err := jobService.GetJobByID(ctx, getReq)

	// --- Assert Get ---
	require.NoError(t, err)
	require.NotNil(t, fetchedJob)
	assert.Equal(t, createdJob.ID, fetchedJob.ID)
	assert.Equal(t, createdJob.Rate, fetchedJob.Rate)
	assert.Equal(t, createdJob.Duration, fetchedJob.Duration)
	assert.Equal(t, createdJob.InvoiceInterval, fetchedJob.InvoiceInterval)
	assert.Equal(t, createdJob.EmployerID, fetchedJob.EmployerID)
	assert.Equal(t, createdJob.State, fetchedJob.State)
	assert.Equal(t, createdJob.ContractorID, fetchedJob.ContractorID)
	// Compare times loosely or truncate
	assert.WithinDuration(t, createdJob.CreatedAt, fetchedJob.CreatedAt, time.Second)
	assert.WithinDuration(t, createdJob.UpdatedAt, fetchedJob.UpdatedAt, time.Second)

	// --- Get Job By ID - Not Found ---
	getReqNotFound := &dto.GetJobByIDRequest{ID: uuid.New()}
	_, err = jobService.GetJobByID(ctx, getReqNotFound)
	require.Error(t, err)
	assert.True(t, errors.Is(err, services.ErrNotFound))
}

func TestJobService_Integration_UpdateJobDetails(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	jobRepo := postgres.NewJobRepo(pool) // Need for verification
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	employer := createTestUser(t, ctx, pool, "updatejob-employer@test.com", "UpdateJob Employer")
	otherUser := createTestUser(t, ctx, pool, "updatejob-other@test.com", "UpdateJob Other")
	contractor := createTestUser(t, ctx, pool, "updatejob-contractor@test.com", "UpdateJob Contractor")

	// Job that can be updated
	jobWaiting := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
	// Job that cannot be updated (wrong state)
	jobOngoing := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)
	// Job that cannot be updated (contractor assigned, even if waiting - though unlikely state)
	jobWaitingWithContractor := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, &contractor.ID)

	tests := []struct {
		name          string
		req           *dto.UpdateJobDetailsRequest
		targetJobID   uuid.UUID
		expectedRate  float64
		expectedDur   int
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.UpdateJobDetailsRequest{
				UserID:   employer.ID, // Correct user
				Rate:     ptrFloat64(125.50),
				Duration: ptrInt(50),
			},
			targetJobID:  jobWaiting.ID,
			expectedRate: 125.50,
			expectedDur:  50,
			expectedErr:  nil,
		},
		{
			name: "Success_OnlyRate",
			req: &dto.UpdateJobDetailsRequest{
				UserID: employer.ID,
				Rate:   ptrFloat64(99.99),
			},
			targetJobID:  jobWaiting.ID,
			expectedRate: 99.99,
			expectedDur:  50, // First success will update it to 50
			expectedErr:  nil,
		},
		{
			name: "Error_Forbidden_WrongUser",
			req: &dto.UpdateJobDetailsRequest{
				UserID: otherUser.ID, // Wrong user
				Rate:   ptrFloat64(130.0),
			},
			targetJobID: jobWaiting.ID,
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_Forbidden_WrongState",
			req: &dto.UpdateJobDetailsRequest{
				UserID: employer.ID,
				Rate:   ptrFloat64(130.0),
			},
			targetJobID: jobOngoing.ID, // Job is Ongoing
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_Forbidden_ContractorAssigned",
			req: &dto.UpdateJobDetailsRequest{
				UserID: employer.ID,
				Rate:   ptrFloat64(130.0),
			},
			targetJobID: jobWaitingWithContractor.ID, // Contractor assigned
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_JobNotFound",
			req: &dto.UpdateJobDetailsRequest{
				UserID: employer.ID,
				Rate:   ptrFloat64(130.0),
			},
			targetJobID: uuid.New(),           // Non-existent job
			expectedErr: services.ErrNotFound, // mapRepoError maps this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fetch initial state for comparison if needed, especially for success cases
			initialJob, _ := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: tt.targetJobID})

			tt.req.JobID = tt.targetJobID // Set JobID for the request

			updatedJob, err := jobService.UpdateJobDetails(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedJob)

				// Verify job didn't change in DB
				if initialJob != nil { // Only check if the job existed initially
					dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: tt.targetJobID})
					require.NoError(t, dbErr)
					assert.Equal(t, initialJob.Rate, dbJob.Rate)
					assert.Equal(t, initialJob.Duration, dbJob.Duration)
					assert.Equal(t, initialJob.State, dbJob.State)
					assert.Equal(t, initialJob.ContractorID, dbJob.ContractorID)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedJob)
				assert.Equal(t, tt.targetJobID, updatedJob.ID)
				assert.Equal(t, tt.expectedRate, updatedJob.Rate)
				assert.Equal(t, tt.expectedDur, updatedJob.Duration)
				assert.Equal(t, job.StateWaiting, updatedJob.State) // Should remain waiting
				assert.Equal(t, uuid.Nil, updatedJob.ContractorID)  // Should remain nil
				require.NotNil(t, initialJob)                       // Should exist for success case
				assert.True(t, updatedJob.UpdatedAt.After(initialJob.UpdatedAt))

				// Verify in DB
				dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: tt.targetJobID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedRate, dbJob.Rate)
				assert.Equal(t, tt.expectedDur, dbJob.Duration)
				assert.Equal(t, job.StateWaiting, dbJob.State)
				assert.Equal(t, uuid.Nil, dbJob.ContractorID)
			}
		})
	}
}

func TestJobService_Integration_UpdateJobState(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	jobRepo := postgres.NewJobRepo(pool) // Need for verification
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	employer := createTestUser(t, ctx, pool, "updstate-employer@test.com", "UpdState Employer")
	contractor := createTestUser(t, ctx, pool, "updstate-contractor@test.com", "UpdState Contractor")
	otherUser := createTestUser(t, ctx, pool, "updstate-other@test.com", "UpdState Other")

	// We need fresh jobs for each state transition test to ensure isolation
	createJobForTest := func(state job.State, contractorID *uuid.UUID) *ent.Job {
		return createTestJob(t, ctx, pool, employer.ID, state, contractorID)
	}

	tests := []struct {
		name          string
		setupFunc     func() uuid.UUID // Returns JobID for the test
		req           *dto.UpdateJobStateRequest
		expectedState job.State
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success_Employer_OngoingToComplete",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateOngoing, &contractor.ID).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: employer.ID,
				State:  job.StateComplete,
			},
			expectedState: job.StateComplete,
			expectedErr:   nil,
		},
		{
			name: "Success_Contractor_OngoingToComplete",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateOngoing, &contractor.ID).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: contractor.ID,
				State:  job.StateComplete,
			},
			expectedState: job.StateComplete,
			expectedErr:   nil,
		},
		{
			name: "Success_Employer_CompleteToArchived",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateComplete, &contractor.ID).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: employer.ID,
				State:  job.StateArchived,
			},
			expectedState: job.StateArchived,
			expectedErr:   nil,
		},
		{
			name: "Error_Forbidden_OtherUser",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateOngoing, &contractor.ID).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: otherUser.ID, // Wrong user
				State:  job.StateComplete,
			},
			expectedState: job.StateOngoing, // Should remain unchanged
			expectedErr:   services.ErrForbidden,
		},
		{
			name: "Error_InvalidTransition_CompleteToWaiting",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateComplete, &contractor.ID).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: employer.ID,
				State:  job.StateWaiting, // Invalid transition
			},
			expectedState: job.StateComplete, // Should remain unchanged
			expectedErr:   services.ErrInvalidTransition,
		},
		{
			name: "Error_InvalidTransition_ManualWaitingToOngoing",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateWaiting, nil).ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: employer.ID,
				State:  job.StateOngoing, // Manual attempt
			},
			expectedState: job.StateWaiting, // Should remain unchanged
			expectedErr:   services.ErrInvalidTransition,
			errorContains: "cannot manually set state to Ongoing",
		},
		{
			name: "Error_JobNotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.UpdateJobStateRequest{
				UserID: employer.ID,
				State:  job.StateComplete,
			},
			expectedErr: services.ErrNotFound, // mapRepoError maps this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetJobID := tt.setupFunc() // Create/get the job for this specific test run
			tt.req.JobID = targetJobID

			updatedJob, err := jobService.UpdateJobState(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedJob)

				// Verify job state didn't change in DB (if it existed)
				dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
				if !errors.Is(tt.expectedErr, services.ErrNotFound) { // Only check if job should exist
					require.NoError(t, dbErr)
					assert.Equal(t, tt.expectedState, dbJob.State)
				} else {
					require.Error(t, dbErr) // Should not find if original error was NotFound
					assert.True(t, errors.Is(dbErr, storage.ErrNotFound))
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedJob)
				assert.Equal(t, targetJobID, updatedJob.ID)
				assert.Equal(t, tt.expectedState, updatedJob.State)

				// Verify in DB
				dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedState, dbJob.State)
			}
		})
	}
}

func TestJobService_Integration_DeleteJob(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	jobRepo := postgres.NewJobRepo(pool) // Need for verification
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	employer := createTestUser(t, ctx, pool, "deletejob-employer@test.com", "DeleteJob Employer")
	otherUser := createTestUser(t, ctx, pool, "deletejob-other@test.com", "DeleteJob Other")
	contractor := createTestUser(t, ctx, pool, "deletejob-contractor@test.com", "DeleteJob Contractor")

	// We need fresh jobs for each state transition test to ensure isolation
	createJobForTest := func(state job.State, contractorID *uuid.UUID) *ent.Job {
		return createTestJob(t, ctx, pool, employer.ID, state, contractorID)
	}

	tests := []struct {
		name        string
		setupFunc   func() uuid.UUID // Returns JobID for the test
		req         *dto.DeleteJobRequest
		expectedErr error
	}{
		{
			name: "Success",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateWaiting, nil).ID
			},
			req: &dto.DeleteJobRequest{
				UserID: employer.ID, // Correct user
			},
			expectedErr: nil,
		},
		{
			name: "Error_Forbidden_NotEmployer",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateWaiting, nil).ID
			},
			req: &dto.DeleteJobRequest{
				UserID: otherUser.ID, // Wrong user
			},
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_InvalidState_NotWaiting",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateOngoing, &contractor.ID).ID
			},
			req: &dto.DeleteJobRequest{
				UserID: employer.ID,
			},
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "Error_InvalidState_ContractorAssigned",
			setupFunc: func() uuid.UUID {
				return createJobForTest(job.StateWaiting, &contractor.ID).ID
			},
			req: &dto.DeleteJobRequest{
				UserID: employer.ID,
			},
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "Error_JobNotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.DeleteJobRequest{
				UserID: employer.ID,
			},
			expectedErr: services.ErrNotFound, // mapRepoError maps this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetJobID := tt.setupFunc() // Create/get the job for this specific test run
			tt.req.ID = targetJobID

			err := jobService.DeleteJob(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)

				// Verify job still exists in DB (if it wasn't a NotFound error initially)
				if !errors.Is(tt.expectedErr, services.ErrNotFound) {
					_, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
					assert.NoError(t, dbErr, "Job should still exist after failed delete")
				}
			} else {
				require.NoError(t, err)

				// Verify job is gone from DB
				_, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
				require.Error(t, dbErr)
				assert.True(t, errors.Is(dbErr, storage.ErrNotFound), "Job should be deleted")
			}
		})
	}
}

// TestJobService_Integration_ListAvailableJobs tests listing available jobs with filters.
func TestJobService_Integration_ListAvailableJobs(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	// --- Setup Data ---
	emp1 := createTestUser(t, ctx, pool, "listavail-emp1@test.com", "ListAvail Emp1")
	emp2 := createTestUser(t, ctx, pool, "listavail-emp2@test.com", "ListAvail Emp2")

	// Create jobs with different states and rates
	job1WaitingLowRate := createTestJob(t, ctx, pool, emp1.ID, job.StateWaiting, nil) // Rate 50.0
	job2WaitingHighRate := createTestJob(t, ctx, pool, emp2.ID, job.StateWaiting, nil)
	_, _ = postgres.NewJobRepo(pool).Update(ctx, &dto.UpdateJobRequest{ID: job2WaitingHighRate.ID, Rate: ptrFloat64(150.0)}) // Update rate
	job4WaitingMidRate := createTestJob(t, ctx, pool, emp1.ID, job.StateWaiting, nil)
	_, _ = postgres.NewJobRepo(pool).Update(ctx, &dto.UpdateJobRequest{ID: job4WaitingMidRate.ID, Rate: ptrFloat64(100.0)}) // Update rate

	// --- Test Cases ---
	tests := []struct {
		name          string
		req           dto.ListAvailableJobsRequest
		expectedCount int
		expectedIDs   []uuid.UUID // Check specific IDs returned
	}{
		{
			name:          "ListAllAvailable",
			req:           dto.ListAvailableJobsRequest{Limit: 10, Offset: 0},
			expectedCount: 3, // job1, job2, job4
			expectedIDs:   []uuid.UUID{job1WaitingLowRate.ID, job2WaitingHighRate.ID, job4WaitingMidRate.ID},
		},
		{
			name:          "FilterMinRate",
			req:           dto.ListAvailableJobsRequest{Limit: 10, Offset: 0, MinRate: ptrFloat64(75.0)},
			expectedCount: 2, // job2, job4
			expectedIDs:   []uuid.UUID{job2WaitingHighRate.ID, job4WaitingMidRate.ID},
		},
		{
			name:          "FilterMaxRate",
			req:           dto.ListAvailableJobsRequest{Limit: 10, Offset: 0, MaxRate: ptrFloat64(120.0)},
			expectedCount: 2, // job1, job4
			expectedIDs:   []uuid.UUID{job1WaitingLowRate.ID, job4WaitingMidRate.ID},
		},
		{
			name:          "FilterMinAndMaxRate",
			req:           dto.ListAvailableJobsRequest{Limit: 10, Offset: 0, MinRate: ptrFloat64(60.0), MaxRate: ptrFloat64(110.0)},
			expectedCount: 1, // job4
			expectedIDs:   []uuid.UUID{job4WaitingMidRate.ID},
		},
		{
			name:          "Pagination_Limit",
			req:           dto.ListAvailableJobsRequest{Limit: 2, Offset: 0},
			expectedCount: 2,
			// Order is DESC by created_at, so likely job4, job2 (or job1 depending on creation order)
		},
		{
			name:          "Pagination_Offset",
			req:           dto.ListAvailableJobsRequest{Limit: 2, Offset: 1},
			expectedCount: 2,
			// Should get the 2nd and 3rd available jobs based on creation time desc
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, err := jobService.ListAvailableJobs(ctx, &tt.req)
			require.NoError(t, err)
			assert.Len(t, jobs, tt.expectedCount)

			// Verify all returned jobs are indeed available
			for _, j := range jobs {
				assert.Equal(t, job.StateWaiting, j.State)
				assert.Equal(t, uuid.Nil, j.ContractorID)
				// Verify rate filters if applied
				if tt.req.MinRate != nil {
					assert.GreaterOrEqual(t, j.Rate, *tt.req.MinRate)
				}
				if tt.req.MaxRate != nil {
					assert.LessOrEqual(t, j.Rate, *tt.req.MaxRate)
				}
			}

			// Verify specific IDs if provided
			if tt.expectedIDs != nil {
				returnedIDs := make([]uuid.UUID, len(jobs))
				for i, job := range jobs {
					returnedIDs[i] = job.ID
				}
				for _, expectedID := range tt.expectedIDs {
					assert.Contains(t, returnedIDs, expectedID)
				}
			}
		})
	}
}

// TestJobService_Integration_ListJobsByEmployer tests listing jobs for an employer.
func TestJobService_Integration_ListJobsByEmployer(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	// --- Setup Data ---
	emp1 := createTestUser(t, ctx, pool, "listemp-emp1@test.com", "ListEmp Emp1")
	emp2 := createTestUser(t, ctx, pool, "listemp-emp2@test.com", "ListEmp Emp2") // Another employer
	con1 := createTestUser(t, ctx, pool, "listemp-con1@test.com", "ListEmp Con1")

	// Jobs for emp1
	job1Emp1Waiting := createTestJob(t, ctx, pool, emp1.ID, job.StateWaiting, nil)
	job2Emp1Ongoing := createTestJob(t, ctx, pool, emp1.ID, job.StateOngoing, &con1.ID)
	// Job for emp2
	_ = createTestJob(t, ctx, pool, emp2.ID, job.StateWaiting, nil)

	// --- Test Cases ---
	req := dto.ListJobsByEmployerRequest{
		EmployerID: emp1.ID,
		Limit:      10,
		Offset:     0,
	}

	jobs, err := jobService.ListJobsByEmployer(ctx, &req)

	require.NoError(t, err)
	assert.Len(t, jobs, 2) // Should only list jobs for emp1

	foundJob1 := false
	foundJob2 := false
	for _, j := range jobs {
		assert.Equal(t, emp1.ID, j.EmployerID) // Verify employer ID
		if j.ID == job1Emp1Waiting.ID {
			foundJob1 = true
			assert.Equal(t, job.StateWaiting, j.State)
		}
		if j.ID == job2Emp1Ongoing.ID {
			foundJob2 = true
			assert.Equal(t, job.StateOngoing, j.State)
		}
	}
	assert.True(t, foundJob1, "Waiting job for emp1 not found")
	assert.True(t, foundJob2, "Ongoing job for emp1 not found")
}

// TestJobService_Integration_ListJobsByContractor tests listing jobs for a contractor.
func TestJobService_Integration_ListJobsByContractor(t *testing.T) {
	ctx, jobService, pool := setupJobServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs")

	// --- Setup Data ---
	emp1 := createTestUser(t, ctx, pool, "listcon-emp1@test.com", "ListCon Emp1")
	con1 := createTestUser(t, ctx, pool, "listcon-con1@test.com", "ListCon Con1")
	con2 := createTestUser(t, ctx, pool, "listcon-con2@test.com", "ListCon Con2") // Another contractor

	// Jobs for con1
	job1Con1Ongoing := createTestJob(t, ctx, pool, emp1.ID, job.StateOngoing, &con1.ID)
	job2Con1Complete := createTestJob(t, ctx, pool, emp1.ID, job.StateComplete, &con1.ID)
	// Job for con2
	_ = createTestJob(t, ctx, pool, emp1.ID, job.StateOngoing, &con2.ID)
	// Unassigned job
	_ = createTestJob(t, ctx, pool, emp1.ID, job.StateWaiting, nil)

	// --- Test Cases ---
	req := dto.ListJobsByContractorRequest{
		ContractorID: con1.ID,
		Limit:        10,
		Offset:       0,
	}

	jobs, err := jobService.ListJobsByContractor(ctx, &req)

	require.NoError(t, err)
	assert.Len(t, jobs, 2) // Should only list jobs for con1

	foundJob1 := false
	foundJob2 := false
	for _, j := range jobs {
		require.NotNil(t, j.ContractorID)
		assert.Equal(t, con1.ID, j.ContractorID) // Verify contractor ID
		if j.ID == job1Con1Ongoing.ID {
			foundJob1 = true
			assert.Equal(t, job.StateOngoing, j.State)
		}
		if j.ID == job2Con1Complete.ID {
			foundJob2 = true
			assert.Equal(t, job.StateComplete, j.State)
		}
	}
	assert.True(t, foundJob1, "Ongoing job for con1 not found")
	assert.True(t, foundJob2, "Complete job for con1 not found")
}
