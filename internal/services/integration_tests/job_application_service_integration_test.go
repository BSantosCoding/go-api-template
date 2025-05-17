package integration_tests

import (
	"context"
	"errors"
	"testing"

	"go-api-template/ent"
	"go-api-template/ent/job"
	"go-api-template/ent/jobapplication"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"          // For storage errors
	"go-api-template/internal/storage/postgres" // Need concrete repos for setup/assertion
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test Setup ---

// setupJobApplicationServiceIntegrationTest initializes the service with a real DB pool.
func setupJobApplicationServiceIntegrationTest(t *testing.T) (context.Context, services.JobApplicationService, *ent.Client) {
	t.Helper()
	pool, _ := getTestClients(t)
	// Instantiate the real service
	jobAppService := services.NewJobApplicationService(pool)
	ctx := context.Background()
	return ctx, jobAppService, pool
}

// Helper function to create an application for tests
func createTestApplication(t *testing.T, ctx context.Context, pool *ent.Client, jobID, contractorID uuid.UUID, state jobapplication.State) *ent.JobApplication {
	t.Helper()
	appRepo := postgres.NewJobApplicationRepo(pool)
	appReq := &dto.CreateJobApplicationRequest{
		JobID:        jobID,
		ContractorID: contractorID,
	}
	app, err := appRepo.Create(ctx, appReq)
	if err != nil && errors.Is(err, storage.ErrConflict) {
		require.NoError(t, err, "Failed to create test application (or handle existing)")
	} else {
		require.NoError(t, err, "Failed to create test application")
	}
	require.NotNil(t, app)

	if state != jobapplication.StateWaiting && app.State != state {
		updateReq := dto.UpdateJobApplicationStateRequest{ID: app.ID, State: state}
		updatedApp, updateErr := appRepo.UpdateState(ctx, &updateReq)
		require.NoError(t, updateErr, "Failed to update test application state")
		return updatedApp
	}
	return app
}

// --- Test Cases ---

func TestJobApplicationService_Integration_ApplyToJob(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	appRepo := postgres.NewJobApplicationRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer := createTestUser(t, ctx, pool, "apply-employer@test.com", "Apply Employer")
	contractor := createTestUser(t, ctx, pool, "apply-contractor@test.com", "Apply Contractor")
	jobWaiting := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
	jobOngoing := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID) // Create an ongoing job for failure case

	tests := []struct {
		name          string
		req           *dto.ApplyToJobRequest
		expectedState jobapplication.State
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success",
			req: &dto.ApplyToJobRequest{
				JobID:        jobWaiting.ID,
				ContractorID: contractor.ID,
			},
			expectedState: jobapplication.StateWaiting,
			expectedErr:   nil,
		},
		{
			name: "Error_JobNotFound",
			req: &dto.ApplyToJobRequest{
				JobID:        uuid.New(), // Non-existent job
				ContractorID: contractor.ID,
			},
			expectedErr:   services.ErrNotFound,
			errorContains: "fetching job",
		},
		{
			name: "Error_JobNotWaiting",
			req: &dto.ApplyToJobRequest{
				JobID:        jobOngoing.ID, // Job is ongoing
				ContractorID: contractor.ID,
			},
			expectedErr:   services.ErrInvalidState,
			errorContains: "job is not available for applications",
		},
		{
			name: "Error_EmployerApplying",
			req: &dto.ApplyToJobRequest{
				JobID:        jobWaiting.ID,
				ContractorID: employer.ID, // Employer tries to apply
			},
			expectedErr:   services.ErrForbidden,
			errorContains: "employer cannot apply",
		},
		{
			name: "Error_AlreadyApplied",
			req: &dto.ApplyToJobRequest{
				JobID:        jobWaiting.ID,
				ContractorID: contractor.ID, // Same as first success case
			},
			expectedErr:   services.ErrConflict,
			errorContains: "already applied",
		},
	}

	// Run success case first to ensure application exists for duplicate check
	t.Run(tests[0].name, func(t *testing.T) {
		tt := tests[0]
		application, err := jobAppService.ApplyToJob(ctx, tt.req)
		require.NoError(t, err)
		require.NotNil(t, application)
		assert.Equal(t, tt.req.JobID, application.JobID)
		assert.Equal(t, tt.req.ContractorID, application.ContractorID)
		assert.Equal(t, tt.expectedState, application.State)
		assert.NotEqual(t, uuid.Nil, application.ID)
		// Verify in DB
		getReq := &dto.GetJobApplicationByIDRequest{ID: application.ID}
		dbApp, dbErr := appRepo.GetByID(ctx, getReq)
		require.NoError(t, dbErr)
		require.NotNil(t, dbApp)
		assert.Equal(t, application.ID, dbApp.ID)
		assert.Equal(t, application.State, dbApp.State)
	})

	// Run remaining tests
	for _, tt := range tests[1:] {
		t.Run(tt.name, func(t *testing.T) {
			application, err := jobAppService.ApplyToJob(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, application)
			} else {
				// This block should ideally not be reached for error tests,
				// but included for completeness if a test case is misconfigured.
				require.NoError(t, err)
				require.NotNil(t, application)
				assert.Equal(t, tt.req.JobID, application.JobID)
				assert.Equal(t, tt.req.ContractorID, application.ContractorID)
				assert.Equal(t, tt.expectedState, application.State)
			}
		})
	}
}

func TestJobApplicationService_Integration_AcceptApplication(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	jobRepo := postgres.NewJobRepo(pool)            // For verification
	appRepo := postgres.NewJobApplicationRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	// Create users once for all tests in this function
	employer := createTestUser(t, ctx, pool, "accept-employer@test.com", "Accept Employer")
	contractor1 := createTestUser(t, ctx, pool, "accept-contractor1@test.com", "Accept Contractor 1")
	contractor2 := createTestUser(t, ctx, pool, "accept-contractor2@test.com", "Accept Contractor 2")
	otherUser := createTestUser(t, ctx, pool, "accept-other@test.com", "Accept Other")

	// --- Define Test Scenarios ---
	// We will create specific jobs/apps inside each test case for isolation

	tests := []struct {
		name                 string
		setupFunc            func() (targetAppID, otherAppID, targetJobID uuid.UUID) // Returns IDs needed for the test
		req                  *dto.AcceptApplicationRequest
		expectedJobState     job.State
		expectedContractorID uuid.UUID
		expectedApp1State    jobapplication.State // State of app being accepted/targeted
		expectedApp2State    jobapplication.State // State of the *other* waiting app (if applicable)
		expectedErr          error
		errorContains        string
	}{
		{
			name: "Success",
			setupFunc: func() (uuid.UUID, uuid.UUID, uuid.UUID) {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app1 := createTestApplication(t, ctx, pool, job.ID, contractor1.ID, jobapplication.StateWaiting)
				app2 := createTestApplication(t, ctx, pool, job.ID, contractor2.ID, jobapplication.StateWaiting)
				return app1.ID, app2.ID, job.ID
			},
			req: &dto.AcceptApplicationRequest{
				// ApplicationID set by setupFunc
				UserID: employer.ID, // Correct employer
			},
			expectedJobState:     job.StateOngoing,
			expectedContractorID: contractor1.ID,
			expectedApp1State:    jobapplication.StateAccepted,
			expectedApp2State:    jobapplication.StateRejected, // Other app should be rejected
			expectedErr:          nil,
		},
		{
			name: "Error_Forbidden_NotEmployer",
			setupFunc: func() (uuid.UUID, uuid.UUID, uuid.UUID) {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app1 := createTestApplication(t, ctx, pool, job.ID, contractor1.ID, jobapplication.StateWaiting)
				return app1.ID, uuid.Nil, job.ID // No other app needed here
			},
			req: &dto.AcceptApplicationRequest{
				// ApplicationID set by setupFunc
				UserID: otherUser.ID, // Wrong user
			},
			// Expect no changes
			expectedJobState:     job.StateWaiting, // Remains waiting
			expectedContractorID: uuid.Nil,
			expectedApp1State:    jobapplication.StateWaiting, // Should remain waiting
			expectedApp2State:    jobapplication.StateWaiting, // Not applicable here, but default check
			expectedErr:          services.ErrForbidden,
		},
		{
			name: "Error_JobNotWaiting",
			setupFunc: func() (uuid.UUID, uuid.UUID, uuid.UUID) {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor1.ID) // Job already taken
				app := createTestApplication(t, ctx, pool, job.ID, contractor2.ID, jobapplication.StateWaiting)
				return app.ID, uuid.Nil, job.ID
			},
			req: &dto.AcceptApplicationRequest{
				// ApplicationID set by setupFunc
				UserID: employer.ID,
			},
			expectedErr:   services.ErrInvalidState,
			errorContains: "job is not in a state to accept applications",
		},
		{
			name: "Error_ApplicationNotWaiting",
			setupFunc: func() (uuid.UUID, uuid.UUID, uuid.UUID) {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor1.ID, jobapplication.StateAccepted) // App already accepted
				return app.ID, uuid.Nil, job.ID
			},
			req: &dto.AcceptApplicationRequest{
				// ApplicationID set by setupFunc
				UserID: employer.ID,
			},
			expectedErr:   services.ErrInvalidState,
			errorContains: "application is not in 'Waiting' state",
		},
		{
			name: "Error_ApplicationNotFound",
			setupFunc: func() (uuid.UUID, uuid.UUID, uuid.UUID) {
				// No need to create app, just return non-existent ID
				return uuid.New(), uuid.Nil, uuid.New() // Return non-existent app ID and dummy job ID
			},
			req: &dto.AcceptApplicationRequest{
				// ApplicationID set by setupFunc
				UserID: employer.ID,
			},
			expectedErr:   services.ErrNotFound,
			errorContains: "fetching application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var targetAppID, otherAppID, targetJobID uuid.UUID
			// Get initial state defaults for verification on failure
			initialJobState := job.StateWaiting
			initialContractorID := uuid.Nil
			initialApp1State := jobapplication.StateWaiting
			initialApp2State := jobapplication.StateWaiting

			if tt.setupFunc != nil {
				targetAppID, otherAppID, targetJobID = tt.setupFunc()
				tt.req.ApplicationID = targetAppID // Set the correct ID for the request

				// Fetch initial state accurately if the job/app exists
				if targetJobID != uuid.Nil {
					initialJob, err := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
					if err == nil {
						initialJobState = initialJob.State
						initialContractorID = initialJob.ContractorID
					}
				}
				if targetAppID != uuid.Nil {
					initialApp1, err := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
					if err == nil {
						initialApp1State = initialApp1.State
					}
				}
				if otherAppID != uuid.Nil {
					initialApp2, err := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: otherAppID})
					if err == nil {
						initialApp2State = initialApp2.State
					}
				}
			} else {
				// Handle cases like ApplicationNotFound where setup isn't strictly needed
				targetAppID = tt.req.ApplicationID // Use the one from the test definition
				targetJobID = uuid.New()           // Dummy job ID for verification if needed
			}

			updatedJob, err := jobAppService.AcceptApplication(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedJob)

				// Verify job state didn't change unexpectedly
				dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
				if !errors.Is(tt.expectedErr, services.ErrNotFound) { // Only check if job should exist
					require.NoError(t, dbErr, "Job should exist for verification after error")
					assert.Equal(t, initialJobState, dbJob.State, "Job state should not have changed on error")
					assert.Equal(t, initialContractorID, dbJob.ContractorID, "Job contractor should not have changed on error")
				} else if dbErr == nil {
					t.Errorf("Job %s still found after expected NotFound error", targetJobID)
				}

				// Verify app states didn't change unexpectedly
				if targetAppID != uuid.Nil && !errors.Is(tt.expectedErr, services.ErrNotFound) { // Check target app if it should exist
					dbApp1, dbErr1 := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
					require.NoError(t, dbErr1, "Target app should exist for verification after error")
					assert.Equal(t, initialApp1State, dbApp1.State, "Target app state should not have changed on error")
				}
				if otherAppID != uuid.Nil { // Check other app if it exists for this test
					dbApp2, dbErr2 := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: otherAppID})
					require.NoError(t, dbErr2, "Other app should exist for verification after error")
					assert.Equal(t, initialApp2State, dbApp2.State, "Other app state should not have changed on error")
				}

			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedJob)

				// Verify returned job
				assert.Equal(t, targetJobID, updatedJob.ID)
				assert.Equal(t, tt.expectedJobState, updatedJob.State)
				assert.Equal(t, tt.expectedContractorID, updatedJob.ContractorID)

				// Verify job in DB
				dbJob, dbErr := jobRepo.GetByID(ctx, &dto.GetJobByIDRequest{ID: targetJobID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedJobState, dbJob.State)
				assert.Equal(t, tt.expectedContractorID, dbJob.ContractorID)

				// Verify application states in DB
				if targetAppID != uuid.Nil {
					dbApp1, dbErr1 := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
					require.NoError(t, dbErr1)
					assert.Equal(t, tt.expectedApp1State, dbApp1.State)
				}
				if otherAppID != uuid.Nil {
					dbApp2, dbErr2 := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: otherAppID})
					require.NoError(t, dbErr2)
					assert.Equal(t, tt.expectedApp2State, dbApp2.State)
				}
			}
		})
	}
}

// TestJobApplicationService_Integration_RejectApplication tests rejecting an application.
func TestJobApplicationService_Integration_RejectApplication(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	appRepo := postgres.NewJobApplicationRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer := createTestUser(t, ctx, pool, "reject-employer@test.com", "Reject Employer")
	contractor := createTestUser(t, ctx, pool, "reject-contractor@test.com", "Reject Contractor")
	otherUser := createTestUser(t, ctx, pool, "reject-other@test.com", "Reject Other")

	tests := []struct {
		name          string
		setupFunc     func() uuid.UUID // Returns ApplicationID
		req           *dto.RejectApplicationRequest
		expectedState jobapplication.State
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateWaiting)
				return app.ID
			},
			req: &dto.RejectApplicationRequest{
				UserID: employer.ID, // Correct employer
			},
			expectedState: jobapplication.StateRejected,
			expectedErr:   nil,
		},
		{
			name: "Error_Forbidden_NotEmployer",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateWaiting)
				return app.ID
			},
			req: &dto.RejectApplicationRequest{
				UserID: otherUser.ID, // Wrong user
			},
			expectedState: jobapplication.StateWaiting, // Should not change
			expectedErr:   services.ErrForbidden,
		},
		{
			name: "Error_ApplicationNotWaiting",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateAccepted) // Already accepted
				return app.ID
			},
			req: &dto.RejectApplicationRequest{
				UserID: employer.ID,
			},
			expectedState: jobapplication.StateAccepted, // Should not change
			expectedErr:   services.ErrInvalidState,
			errorContains: "application is not in 'Waiting' state",
		},
		{
			name: "Error_ApplicationNotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.RejectApplicationRequest{
				UserID: employer.ID,
			},
			expectedErr: services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetAppID := tt.setupFunc()
			tt.req.ApplicationID = targetAppID

			updatedApp, err := jobAppService.RejectApplication(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedApp)

				// Verify state didn't change in DB (if it existed)
				dbApp, dbErr := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
				if !errors.Is(tt.expectedErr, services.ErrNotFound) {
					require.NoError(t, dbErr)
					assert.Equal(t, tt.expectedState, dbApp.State)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedApp)
				assert.Equal(t, targetAppID, updatedApp.ID)
				assert.Equal(t, tt.expectedState, updatedApp.State)

				// Verify in DB
				dbApp, dbErr := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedState, dbApp.State)
			}
		})
	}
}

// TestJobApplicationService_Integration_WithdrawApplication tests withdrawing an application.
func TestJobApplicationService_Integration_WithdrawApplication(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	appRepo := postgres.NewJobApplicationRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer := createTestUser(t, ctx, pool, "withdraw-employer@test.com", "Withdraw Employer")
	contractor := createTestUser(t, ctx, pool, "withdraw-contractor@test.com", "Withdraw Contractor")
	otherUser := createTestUser(t, ctx, pool, "withdraw-other@test.com", "Withdraw Other")

	tests := []struct {
		name          string
		setupFunc     func() uuid.UUID // Returns ApplicationID
		req           *dto.WithdrawApplicationRequest
		expectedState jobapplication.State
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateWaiting)
				return app.ID
			},
			req: &dto.WithdrawApplicationRequest{
				UserID: contractor.ID, // Correct applicant
			},
			expectedState: jobapplication.StateWithdrawn,
			expectedErr:   nil,
		},
		{
			name: "Error_Forbidden_NotApplicant",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateWaiting)
				return app.ID
			},
			req: &dto.WithdrawApplicationRequest{
				UserID: otherUser.ID, // Wrong user
			},
			expectedState: jobapplication.StateWaiting, // Should not change
			expectedErr:   services.ErrForbidden,
		},
		{
			name: "Error_ApplicationNotWaiting",
			setupFunc: func() uuid.UUID {
				job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
				app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateRejected) // Already rejected
				return app.ID
			},
			req: &dto.WithdrawApplicationRequest{
				UserID: contractor.ID,
			},
			expectedState: jobapplication.StateRejected, // Should not change
			expectedErr:   services.ErrInvalidState,
			errorContains: "application is not in 'Waiting' state",
		},
		{
			name: "Error_ApplicationNotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.WithdrawApplicationRequest{
				UserID: contractor.ID,
			},
			expectedErr: services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetAppID := tt.setupFunc()
			tt.req.ApplicationID = targetAppID

			updatedApp, err := jobAppService.WithdrawApplication(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedApp)

				// Verify state didn't change in DB (if it existed)
				dbApp, dbErr := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
				if !errors.Is(tt.expectedErr, services.ErrNotFound) {
					require.NoError(t, dbErr)
					assert.Equal(t, tt.expectedState, dbApp.State)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedApp)
				assert.Equal(t, targetAppID, updatedApp.ID)
				assert.Equal(t, tt.expectedState, updatedApp.State)

				// Verify in DB
				dbApp, dbErr := appRepo.GetByID(ctx, &dto.GetJobApplicationByIDRequest{ID: targetAppID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedState, dbApp.State)
			}
		})
	}
}

// TestJobApplicationService_Integration_GetApplicationByID tests getting an application by ID.
func TestJobApplicationService_Integration_GetApplicationByID(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer := createTestUser(t, ctx, pool, "getapp-employer@test.com", "GetApp Employer")
	contractor := createTestUser(t, ctx, pool, "getapp-contractor@test.com", "GetApp Contractor")
	otherUser := createTestUser(t, ctx, pool, "getapp-other@test.com", "GetApp Other")
	job := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
	app := createTestApplication(t, ctx, pool, job.ID, contractor.ID, jobapplication.StateWaiting)

	tests := []struct {
		name        string
		req         *dto.GetJobApplicationByIDRequest
		expectedErr error
	}{
		{
			name:        "Success_AsApplicant",
			req:         &dto.GetJobApplicationByIDRequest{ID: app.ID, UserID: contractor.ID},
			expectedErr: nil,
		},
		{
			name:        "Success_AsEmployer",
			req:         &dto.GetJobApplicationByIDRequest{ID: app.ID, UserID: employer.ID},
			expectedErr: nil,
		},
		{
			name:        "Error_Forbidden",
			req:         &dto.GetJobApplicationByIDRequest{ID: app.ID, UserID: otherUser.ID},
			expectedErr: services.ErrForbidden,
		},
		{
			name:        "Error_NotFound",
			req:         &dto.GetJobApplicationByIDRequest{ID: uuid.New(), UserID: contractor.ID},
			expectedErr: services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchedApp, err := jobAppService.GetApplicationByID(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				assert.Nil(t, fetchedApp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, fetchedApp)
				assert.Equal(t, tt.req.ID, fetchedApp.ID)
				assert.Equal(t, job.ID, fetchedApp.JobID)
				assert.Equal(t, contractor.ID, fetchedApp.ContractorID)
			}
		})
	}
}

// TestJobApplicationService_Integration_ListApplicationsByContractor tests listing applications for a contractor.
func TestJobApplicationService_Integration_ListApplicationsByContractor(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer1 := createTestUser(t, ctx, pool, "listcon-emp1@test.com", "ListCon Emp1")
	contractor1 := createTestUser(t, ctx, pool, "listcon-con1@test.com", "ListCon Con1")
	contractor2 := createTestUser(t, ctx, pool, "listcon-con2@test.com", "ListCon Con2")

	job1 := createTestJob(t, ctx, pool, employer1.ID, job.StateWaiting, nil)
	job2 := createTestJob(t, ctx, pool, employer1.ID, job.StateWaiting, nil)

	// Apps for contractor1
	app1Con1 := createTestApplication(t, ctx, pool, job1.ID, contractor1.ID, jobapplication.StateWaiting)
	app2Con1 := createTestApplication(t, ctx, pool, job2.ID, contractor1.ID, jobapplication.StateAccepted)
	// App for contractor2
	_ = createTestApplication(t, ctx, pool, job1.ID, contractor2.ID, jobapplication.StateWaiting)

	req := &dto.ListJobApplicationsByContractorRequest{
		ContractorID: contractor1.ID,
		Limit:        10,
		Offset:       0,
	}

	apps, err := jobAppService.ListApplicationsByContractor(ctx, req)

	require.NoError(t, err)
	assert.Len(t, apps, 2)

	// Check if the correct applications were returned (order might vary based on DB)
	foundApp1 := false
	foundApp2 := false
	for _, app := range apps {
		assert.Equal(t, contractor1.ID, app.ContractorID)
		if app.ID == app1Con1.ID {
			foundApp1 = true
			assert.Equal(t, jobapplication.StateWaiting, app.State)
		}
		if app.ID == app2Con1.ID {
			foundApp2 = true
			assert.Equal(t, jobapplication.StateAccepted, app.State)
		}
	}
	assert.True(t, foundApp1, "Application 1 for contractor 1 not found")
	assert.True(t, foundApp2, "Application 2 for contractor 1 not found")
}

// TestJobApplicationService_Integration_ListApplicationsByJob tests listing applications for a job.
func TestJobApplicationService_Integration_ListApplicationsByJob(t *testing.T) {
	ctx, jobAppService, pool := setupJobApplicationServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs", "job_application")

	employer := createTestUser(t, ctx, pool, "listjob-emp@test.com", "ListJob Emp")
	contractor1 := createTestUser(t, ctx, pool, "listjob-con1@test.com", "ListJob Con1")
	contractor2 := createTestUser(t, ctx, pool, "listjob-con2@test.com", "ListJob Con2")
	otherUser := createTestUser(t, ctx, pool, "listjob-other@test.com", "ListJob Other")

	job1 := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil)
	job2 := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, nil) // Another job by same employer

	// Apps for job1
	app1Job1 := createTestApplication(t, ctx, pool, job1.ID, contractor1.ID, jobapplication.StateWaiting)
	app2Job1 := createTestApplication(t, ctx, pool, job1.ID, contractor2.ID, jobapplication.StateRejected)
	// App for job2
	_ = createTestApplication(t, ctx, pool, job2.ID, contractor1.ID, jobapplication.StateWaiting)

	tests := []struct {
		name          string
		req           *dto.ListJobApplicationsByJobRequest
		expectedCount int
		expectedErr   error
	}{
		{
			name: "Success_AsEmployer",
			req: &dto.ListJobApplicationsByJobRequest{
				JobID:  job1.ID,
				UserID: employer.ID,
				Limit:  10, Offset: 0,
			},
			expectedCount: 2,
			expectedErr:   nil,
		},
		{
			name: "Error_Forbidden_NotEmployer",
			req: &dto.ListJobApplicationsByJobRequest{
				JobID:  job1.ID,
				UserID: otherUser.ID, // Wrong user
				Limit:  10, Offset: 0,
			},
			expectedCount: 0,
			expectedErr:   services.ErrForbidden,
		},
		{
			name: "Error_JobNotFound",
			req: &dto.ListJobApplicationsByJobRequest{
				JobID:  uuid.New(), // Non-existent job
				UserID: employer.ID,
				Limit:  10, Offset: 0,
			},
			expectedCount: 0,
			expectedErr:   services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apps, err := jobAppService.ListApplicationsByJob(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				assert.Nil(t, apps)
			} else {
				require.NoError(t, err)
				assert.Len(t, apps, tt.expectedCount)
				// Verify apps belong to the correct job
				for _, app := range apps {
					assert.Equal(t, tt.req.JobID, app.JobID)
				}
				// Optionally verify specific apps if count > 0
				if tt.name == "Success_AsEmployer" {
					foundApp1 := false
					foundApp2 := false
					for _, app := range apps {
						if app.ID == app1Job1.ID {
							foundApp1 = true
						}
						if app.ID == app2Job1.ID {
							foundApp2 = true
						}
					}
					assert.True(t, foundApp1, "App1 for job1 not found")
					assert.True(t, foundApp2, "App2 for job1 not found")
				}
			}
		})
	}
}
