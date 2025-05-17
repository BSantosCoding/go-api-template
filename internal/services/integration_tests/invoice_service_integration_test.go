package integration_tests

import (
	"context"
	"errors"
	"testing"

	"go-api-template/ent"
	"go-api-template/ent/invoice"
	"go-api-template/ent/job"
	"go-api-template/internal/services"
	"go-api-template/internal/storage"          // For storage errors
	"go-api-template/internal/storage/postgres" // Need concrete repos for setup/assertion
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Helper Functions ---

func ptrInvoiceState(state invoice.State) *invoice.State {
	return &state
}

// --- Test Setup ---

// setupInvoiceServiceIntegrationTest initializes the service with a real DB pool.
func setupInvoiceServiceIntegrationTest(t *testing.T) (context.Context, services.InvoiceService, *ent.Client) {
	t.Helper()
	pool, _ := getTestClients(t)
	// Instantiate the real service
	invoiceService := services.NewInvoiceService(pool)
	ctx := context.Background()
	return ctx, invoiceService, pool
}

// Helper function to create an invoice for tests
func createTestInvoice(t *testing.T, ctx context.Context, pool *ent.Client, jobID uuid.UUID, interval int, value float64, state invoice.State) *ent.Invoice {
	t.Helper()
	invoiceRepo := postgres.NewInvoiceRepo(pool)
	invoice := &ent.Invoice{
		JobID:          jobID,
		IntervalNumber: interval,
		Value:          value,
		State:          state,
	}
	createdInvoice, err := invoiceRepo.Create(ctx, invoice)
	// Handle potential conflict during setup gracefully if needed, or fail test
	require.NoError(t, err, "Failed to create test invoice for job %s, interval %d", jobID, interval)
	require.NotNil(t, createdInvoice)
	return createdInvoice
}

// --- Test Cases ---

func TestInvoiceService_Integration_CreateInvoice(t *testing.T) {
	ctx, invoiceService, pool := setupInvoiceServiceIntegrationTest(t)
	require.NotNil(t, pool, "Database client should not be nil after getTestClients")
	invoiceRepo := postgres.NewInvoiceRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "invoices")

	employer := createTestUser(t, ctx, pool, "invoice-employer@test.com", "Invoice Employer")
	contractor := createTestUser(t, ctx, pool, "invoice-contractor@test.com", "Invoice Contractor")
	otherUser := createTestUser(t, ctx, pool, "invoice-other@test.com", "Invoice Other")

	jobOngoing := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)
	jobWaiting := createTestJob(t, ctx, pool, employer.ID, job.StateWaiting, &contractor.ID)
	jobPartial := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)
	jobPartial.Duration = 25 // e.g., 2 full intervals (10) + 1 partial (5)
	_, err := postgres.NewJobRepo(pool).Update(ctx, &dto.UpdateJobRequest{ID: jobPartial.ID, Duration: &jobPartial.Duration})
	require.NoError(t, err)

	tests := []struct {
		name             string
		req              *dto.CreateInvoiceRequest
		targetJobID      uuid.UUID // Job to target for the request
		expectedValue    float64   // Expected calculated value
		expectedInterval int
		expectedErr      error
		errorContains    string
		setupFunc        func() // Optional setup specific to this test case
	}{
		{
			name: "Success_FirstInvoice",
			req: &dto.CreateInvoiceRequest{ // Target jobPartial
				UserId: contractor.ID,
			},
			targetJobID:      jobPartial.ID, // Use the partial job for first invoice test
			expectedValue:    50.0 * 10,     // 50 rate * 10 interval
			expectedInterval: 1,
			expectedErr:      nil,
		},
		{
			name: "Success_SecondInvoice",
			req: &dto.CreateInvoiceRequest{ // Target jobOngoing
				UserId: contractor.ID,
			},
			targetJobID: jobOngoing.ID, // Use the job that already has invoice 1
			setupFunc: func() {
				// Ensure interval 1 exists for jobOngoing
				_ = createTestInvoice(t, ctx, pool, jobOngoing.ID, 1, 500, invoice.StateWaiting)
			},
			expectedValue:    50.0 * 10, // 50 rate * 10 interval
			expectedInterval: 2,
			expectedErr:      nil,
		},
		{
			name: "Success_PartialLastInvoice",
			req: &dto.CreateInvoiceRequest{ // Target jobPartial
				UserId: contractor.ID,
			},
			targetJobID: jobPartial.ID, // Use partial job, assume interval 1 & 2 exist now
			setupFunc: func() {
				// Ensure interval 1 and 2 exist for jobPartial
				_ = createTestInvoice(t, ctx, pool, jobPartial.ID, 2, 500, invoice.StateWaiting)
			},
			expectedValue:    50.0 * 5, // 50 rate * 5 remaining hours
			expectedInterval: 3,
			expectedErr:      nil,
		},
		{
			name: "Error_JobNotFound",
			req: &dto.CreateInvoiceRequest{
				UserId: contractor.ID,
			},
			targetJobID: uuid.New(), // Non-existent job
			expectedErr: services.ErrNotFound,
		},
		{
			name: "Error_Forbidden_NotContractor",
			req: &dto.CreateInvoiceRequest{
				UserId: otherUser.ID, // Wrong user
			},
			targetJobID: jobOngoing.ID,
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_InvalidState_JobNotOngoing",
			req: &dto.CreateInvoiceRequest{
				UserId: contractor.ID, // Correct user, wrong job state
			},
			targetJobID: jobWaiting.ID,
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "Error_IntervalExceeded",
			req: &dto.CreateInvoiceRequest{
				UserId: contractor.ID,
			}, // Target jobPartial
			targetJobID: jobPartial.ID, // Try to create 4th interval (only 3 possible)
			expectedErr: services.ErrInvalidInvoiceInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run specific setup if defined
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			tt.req.JobID = tt.targetJobID // Set the job ID for the request

			invoiceCreated, err := invoiceService.CreateInvoice(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, invoiceCreated)
			} else {
				require.NoError(t, err)
				require.NotNil(t, invoiceCreated)
				assert.Equal(t, tt.targetJobID, invoiceCreated.JobID)
				assert.Equal(t, tt.expectedInterval, invoiceCreated.IntervalNumber)
				assert.Equal(t, tt.expectedValue, invoiceCreated.Value)
				assert.Equal(t, invoice.StateWaiting, invoiceCreated.State)
				assert.NotEqual(t, uuid.Nil, invoiceCreated.ID)

				// Verify in DB
				dbInvoice, dbErr := invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: invoiceCreated.ID})
				require.NoError(t, dbErr)
				require.NotNil(t, dbInvoice)
				assert.Equal(t, invoiceCreated.ID, dbInvoice.ID)
				assert.Equal(t, tt.expectedInterval, dbInvoice.IntervalNumber)
				assert.Equal(t, tt.expectedValue, dbInvoice.Value)
				assert.Equal(t, invoice.StateWaiting, dbInvoice.State)
			}
		})
	}
}

func TestInvoiceService_Integration_GetInvoiceByID(t *testing.T) {
	ctx, invoiceService, pool := setupInvoiceServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs", "invoices")

	employer := createTestUser(t, ctx, pool, "getinv-employer@test.com", "GetInv Employer")
	contractor := createTestUser(t, ctx, pool, "getinv-contractor@test.com", "GetInv Contractor")
	otherUser := createTestUser(t, ctx, pool, "getinv-other@test.com", "GetInv Other")
	job := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)
	invoice := createTestInvoice(t, ctx, pool, job.ID, 1, 500, invoice.StateWaiting) // Use helper

	tests := []struct {
		name        string
		invoiceID   uuid.UUID
		userID      uuid.UUID // User making the request
		expectedErr error
	}{
		{
			name:        "Success_AsEmployer",
			invoiceID:   invoice.ID,
			userID:      employer.ID,
			expectedErr: nil,
		},
		{
			name:        "Success_AsContractor",
			invoiceID:   invoice.ID,
			userID:      contractor.ID,
			expectedErr: nil,
		},
		{
			name:        "Error_Forbidden",
			invoiceID:   invoice.ID,
			userID:      otherUser.ID,
			expectedErr: services.ErrForbidden,
		},
		{
			name:        "Error_NotFound",
			invoiceID:   uuid.New(), // Non-existent ID
			userID:      employer.ID,
			expectedErr: services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &dto.GetInvoiceByIDRequest{ID: tt.invoiceID, UserId: tt.userID}
			fetchedInvoice, err := invoiceService.GetInvoiceByID(ctx, req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				assert.Nil(t, fetchedInvoice)
			} else {
				require.NoError(t, err)
				require.NotNil(t, fetchedInvoice)
				assert.Equal(t, tt.invoiceID, fetchedInvoice.ID)
				assert.Equal(t, job.ID, fetchedInvoice.JobID) // Verify correct job association
			}
		})
	}
}

func TestInvoiceService_Integration_UpdateInvoiceState(t *testing.T) {
	ctx, invoiceService, pool := setupInvoiceServiceIntegrationTest(t)
	invoiceRepo := postgres.NewInvoiceRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "invoices")

	employer := createTestUser(t, ctx, pool, "updinv-employer@test.com", "UpdInv Employer")
	contractor := createTestUser(t, ctx, pool, "updinv-contractor@test.com", "UpdInv Contractor")
	// otherUser := createTestUser(t, ctx, pool, "updinv-other@test.com", "UpdInv Other") // Not needed for these cases
	job := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)

	tests := []struct {
		name          string
		setupFunc     func() uuid.UUID // Function to setup/get the target invoice ID for the test
		req           *dto.UpdateInvoiceStateRequest
		expectedState invoice.State // Expected final state (or initial state if error)
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success_WaitingToComplete",
			setupFunc: func() uuid.UUID {
				// Create a fresh waiting invoice for this test
				return createTestInvoice(t, ctx, pool, job.ID, 1, 500, invoice.StateWaiting).ID
			},
			req: &dto.UpdateInvoiceStateRequest{
				NewState: invoice.StateComplete,
				UserId:   employer.ID, // Correct user
			},
			// targetInvoiceID will be set by setupFunc
			expectedState: invoice.StateComplete,
			expectedErr:   nil,
		},
		{
			name: "Error_Forbidden_NotEmployer",
			setupFunc: func() uuid.UUID {
				// Ensure a waiting invoice exists for this check
				return createTestInvoice(t, ctx, pool, job.ID, 2, 500, invoice.StateWaiting).ID
			},
			req: &dto.UpdateInvoiceStateRequest{
				NewState: invoice.StateComplete,
				UserId:   contractor.ID, // Employer cannot update state
			},
			// targetInvoiceID will be set by setupFunc
			expectedState: invoice.StateWaiting,  // Should not change
			expectedErr:   services.ErrForbidden, // Service correctly forbids non-contractor
		},
		{
			name: "Error_InvalidTransition_CompleteToWaiting",
			setupFunc: func() uuid.UUID {
				// Create a fresh complete invoice for this test
				return createTestInvoice(t, ctx, pool, job.ID, 3, 500, invoice.StateComplete).ID
			},
			req: &dto.UpdateInvoiceStateRequest{
				NewState: invoice.StateWaiting,
				UserId:   employer.ID,
			},
			// targetInvoiceID will be set by setupFunc
			expectedState: invoice.StateComplete, // Should not change
			expectedErr:   services.ErrInvalidTransition,
		},
		{
			name: "Error_NotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.UpdateInvoiceStateRequest{
				NewState: invoice.StateComplete,
				UserId:   contractor.ID,
			},
			// targetInvoiceID will be set by setupFunc
			expectedErr:   services.ErrNotFound,
			errorContains: "getting invoice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID := uuid.Nil
			if tt.setupFunc != nil {
				targetID = tt.setupFunc() // Get/Setup the specific invoice ID for this test run
			}
			tt.req.ID = targetID // Set the ID for the request

			updatedInvoice, err := invoiceService.UpdateInvoiceState(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, updatedInvoice)

				// Verify state didn't change in DB (if it existed)
				dbInvoice, dbErr := invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: targetID})
				if !errors.Is(tt.expectedErr, services.ErrNotFound) { // Only check if the invoice should exist
					require.NoError(t, dbErr, "Invoice should exist for verification after error")
					assert.Equal(t, tt.expectedState, dbInvoice.State, "Invoice state should not have changed on error")
				} else if dbErr == nil {
					t.Errorf("Invoice %s still found after expected NotFound error", targetID)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedInvoice)
				assert.Equal(t, targetID, updatedInvoice.ID)
				assert.Equal(t, tt.expectedState, updatedInvoice.State)

				// Verify in DB
				dbInvoice, dbErr := invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: targetID})
				require.NoError(t, dbErr)
				assert.Equal(t, tt.expectedState, dbInvoice.State)
			}
		})
	}
}

func TestInvoiceService_Integration_DeleteInvoice(t *testing.T) {
	ctx, invoiceService, pool := setupInvoiceServiceIntegrationTest(t)
	invoiceRepo := postgres.NewInvoiceRepo(pool) // For verification
	defer cleanupTables(ctx, t, pool, "users", "jobs", "invoices")

	employer := createTestUser(t, ctx, pool, "delinv-employer@test.com", "DelInv Employer")
	contractor := createTestUser(t, ctx, pool, "delinv-contractor@test.com", "DelInv Contractor")
	// otherUser := createTestUser(t, ctx, pool, "delinv-other@test.com", "DelInv Other") // Not needed
	job := createTestJob(t, ctx, pool, employer.ID, job.StateOngoing, &contractor.ID)

	tests := []struct {
		name          string
		setupFunc     func() uuid.UUID // Function to setup/get the target invoice ID for the test
		req           *dto.DeleteInvoiceRequest
		expectedErr   error
		errorContains string
	}{
		{
			name: "Success",
			setupFunc: func() uuid.UUID {
				return createTestInvoice(t, ctx, pool, job.ID, 1, 500, invoice.StateWaiting).ID
			},
			req: &dto.DeleteInvoiceRequest{
				UserId: contractor.ID, // Correct user
			},
			// targetInvoiceID set by setupFunc
			expectedErr: nil,
		},
		{
			name: "Error_Forbidden_NotContractor",
			setupFunc: func() uuid.UUID {
				// Ensure a waiting invoice exists for this check
				return createTestInvoice(t, ctx, pool, job.ID, 2, 500, invoice.StateWaiting).ID
			},
			req: &dto.DeleteInvoiceRequest{
				UserId: employer.ID, // Employer cannot delete
			},
			// targetInvoiceID set by setupFunc
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_InvalidState_NotWaiting",
			setupFunc: func() uuid.UUID {
				// Ensure a complete invoice exists for this check
				return createTestInvoice(t, ctx, pool, job.ID, 3, 500, invoice.StateComplete).ID
			},
			req: &dto.DeleteInvoiceRequest{
				UserId: contractor.ID, // Correct user, wrong state
			},
			// targetInvoiceID set by setupFunc
			expectedErr: services.ErrInvalidState,
		},
		{ // No setupFunc needed, targetInvoiceID is non-existent
			name: "Error_NotFound",
			setupFunc: func() uuid.UUID {
				return uuid.New() // Non-existent ID
			},
			req: &dto.DeleteInvoiceRequest{
				UserId: contractor.ID,
			},
			// targetInvoiceID set by setupFunc
			expectedErr:   services.ErrNotFound,
			errorContains: "getting invoice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetID := uuid.Nil
			if tt.setupFunc != nil {
				targetID = tt.setupFunc() // Get/Setup the specific invoice ID for this test run
			}
			tt.req.ID = targetID // Set the ID for the request

			err := invoiceService.DeleteInvoice(ctx, tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				// Verify invoice still exists in DB (if it wasn't a NotFound error initially)
				if !errors.Is(tt.expectedErr, services.ErrNotFound) {
					_, dbErr := invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: targetID})
					assert.NoError(t, dbErr, "Invoice should still exist after failed delete")
				}
			} else {
				require.NoError(t, err)

				// Verify invoice is gone from DB
				_, dbErr := invoiceRepo.GetByID(ctx, &dto.GetInvoiceByIDRequest{ID: targetID})
				require.Error(t, dbErr)
				assert.True(t, errors.Is(dbErr, storage.ErrNotFound), "Invoice should be deleted")
			}
		})
	}
}

// TestInvoiceService_Integration_ListInvoicesByJob tests listing invoices for a job.
func TestInvoiceService_Integration_ListInvoicesByJob(t *testing.T) {
	ctx, invoiceService, pool := setupInvoiceServiceIntegrationTest(t)
	defer cleanupTables(ctx, t, pool, "users", "jobs", "invoices")

	// --- Setup Data ---
	employer1 := createTestUser(t, ctx, pool, "listinv-emp1@test.com", "ListInv Employer 1")
	contractor1 := createTestUser(t, ctx, pool, "listinv-con1@test.com", "ListInv Contractor 1")
	otherUser := createTestUser(t, ctx, pool, "listinv-other@test.com", "ListInv Other")

	job1 := createTestJob(t, ctx, pool, employer1.ID, job.StateOngoing, &contractor1.ID)
	_ = createTestInvoice(t, ctx, pool, job1.ID, 1, 500, invoice.StateWaiting)
	_ = createTestInvoice(t, ctx, pool, job1.ID, 2, 500, invoice.StateComplete)
	_ = createTestInvoice(t, ctx, pool, job1.ID, 3, 500, invoice.StateWaiting)

	// Create another job/invoice to ensure filtering works
	employer2 := createTestUser(t, ctx, pool, "listinv-emp2@test.com", "ListInv Employer 2")
	job2 := createTestJob(t, ctx, pool, employer2.ID, job.StateOngoing, &contractor1.ID) // Same contractor
	_ = createTestInvoice(t, ctx, pool, job2.ID, 1, 600, invoice.StateWaiting)

	// --- Test Cases ---
	tests := []struct {
		name           string
		req            dto.ListInvoicesByJobRequest
		expectedCount  int
		expectedStates []invoice.State // Optional: check states if count > 0
		expectedErr    error
		errorContains  string
	}{
		{
			name: "Success_ListAll_AsEmployer",
			req: dto.ListInvoicesByJobRequest{
				JobID:  job1.ID,
				UserId: employer1.ID,
				Limit:  10, Offset: 0,
			},
			expectedCount: 3,
			expectedErr:   nil,
		},
		{
			name: "Success_ListAll_AsContractor",
			req: dto.ListInvoicesByJobRequest{
				JobID:  job1.ID,
				UserId: contractor1.ID,
				Limit:  10, Offset: 0,
			},
			expectedCount: 3,
			expectedErr:   nil,
		},
		{
			name: "Success_FilterStateWaiting",
			req: dto.ListInvoicesByJobRequest{
				JobID:  job1.ID,
				UserId: employer1.ID,
				State:  ptrInvoiceState(invoice.StateWaiting), // Filter by Waiting
				Limit:  10, Offset: 0,
			},
			expectedCount:  2,
			expectedStates: []invoice.State{invoice.StateWaiting, invoice.StateWaiting},
			expectedErr:    nil,
		},
		{
			name: "Success_FilterStateComplete",
			req: dto.ListInvoicesByJobRequest{
				JobID:  job1.ID,
				UserId: employer1.ID,
				State:  ptrInvoiceState(invoice.StateComplete), // Filter by Complete
				Limit:  10, Offset: 0,
			},
			expectedCount:  1,
			expectedStates: []invoice.State{invoice.StateComplete},
			expectedErr:    nil,
		},
		{
			name: "Error_Forbidden",
			req: dto.ListInvoicesByJobRequest{
				JobID:  job1.ID,
				UserId: otherUser.ID, // User not associated
				Limit:  10, Offset: 0,
			},
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_JobNotFound",
			req: dto.ListInvoicesByJobRequest{
				JobID:  uuid.New(), // Non-existent job
				UserId: employer1.ID,
				Limit:  10, Offset: 0,
			},
			expectedErr: services.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invoices, err := invoiceService.ListInvoicesByJob(ctx, &tt.req)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr), "Expected error %v, got %v", tt.expectedErr, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, invoices)
			} else {
				require.NoError(t, err)
				assert.Len(t, invoices, tt.expectedCount)
				if tt.expectedStates != nil {
					require.Equal(t, len(tt.expectedStates), len(invoices), "Mismatch in expected states count")
					for i, inv := range invoices {
						assert.Equal(t, tt.expectedStates[i], inv.State, "Invoice %d has incorrect state", i)
					}
				}
				// Verify that all returned invoices belong to the correct job
				for _, inv := range invoices {
					assert.Equal(t, tt.req.JobID, inv.JobID)
				}
			}
		})
	}
}
