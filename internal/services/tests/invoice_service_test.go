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

func setupInvoiceServiceTest(t *testing.T) (context.Context, services.InvoiceService, *mock_storage.MockInvoiceRepository, *mock_storage.MockJobRepository, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockInvoiceRepo := mock_storage.NewMockInvoiceRepository(ctrl)
	mockJobRepo := mock_storage.NewMockJobRepository(ctrl)
	invoiceService := services.NewInvoiceService(mockInvoiceRepo, mockJobRepo)
	ctx := context.Background()
	return ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl
}

func TestInvoiceService_CreateInvoice(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockInvoiceRepoGetMaxIntervalForJob struct {
		req *dto.GetMaxIntervalForJobRequest
		res int
		err error
	}
	type mockInvoiceRepoCreate struct {
		req *models.Invoice // Use gomock.Any() for matching input
		res *models.Invoice
		err error
	}

	tests := []struct {
		name                                string
		req                                 *dto.CreateInvoiceRequest
		mockJobRepoGetByID                  mockJobRepoGetByID
		mockInvoiceRepoGetMaxIntervalForJob mockInvoiceRepoGetMaxIntervalForJob
		mockInvoiceRepoCreate               mockInvoiceRepoCreate
		expectedInvoice                     *models.Invoice // For successful cases
		expectedErr                         error
		assertInvoice                       func(*testing.T, *models.Invoice, *models.Invoice) // Custom assertion for invoice
	}{
		{
			name: "Success_FullInterval",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Rate:            100.0,
					Duration:        40,
					InvoiceInterval: 10,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 1,
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{
				req: nil, // Use gomock.Any()
				res: &models.Invoice{
					ID:             uuid.New(), // Simulate repo generating ID
					JobID:          uuid.Nil,   
					IntervalNumber: 2,          // Expected next interval
					Value:          100.0 * 10, // Expected value
					State:          models.InvoiceStateWaiting,
					CreatedAt:      time.Now(), // Simulate repo setting time
					UpdatedAt:      time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedInvoice: &models.Invoice{
				ID:             uuid.Nil, 
				JobID:          uuid.Nil, 
				IntervalNumber: 2,
				Value:          100.0 * 10,
				State:          models.InvoiceStateWaiting,
				CreatedAt:      time.Now(), // Will be asserted loosely
				UpdatedAt:      time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertInvoice: func(t *testing.T, expected, actual *models.Invoice) {
				assert.NotEqual(t, uuid.Nil, actual.ID) // Ensure ID was generated
				assert.Equal(t, expected.JobID, actual.JobID)
				assert.Equal(t, expected.IntervalNumber, actual.IntervalNumber)
				assert.Equal(t, expected.Value, actual.Value)
				assert.Equal(t, expected.State, actual.State)
				// Loosely assert time fields
				assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "Success_PartialLastInterval",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Rate:            50.0,
					Duration:        35, // 3 full intervals (10) + 1 partial (5) = 4 intervals total
					InvoiceInterval: 10,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 3, // Previous interval was 3, this is the 4th (last)
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{
				req: nil, // Use gomock.Any()
				res: &models.Invoice{
					ID:             uuid.New(), // Simulate repo generating ID
					JobID:          uuid.Nil,   
					IntervalNumber: 4,          // Expected next interval
					Value:          50.0 * 5,   // Expected value (remainder hours)
					State:          models.InvoiceStateWaiting,
					CreatedAt:      time.Now(), // Simulate repo setting time
					UpdatedAt:      time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedInvoice: &models.Invoice{
				ID:             uuid.Nil, 
				JobID:          uuid.Nil, 
				IntervalNumber: 4,
				Value:          50.0 * 5,
				State:          models.InvoiceStateWaiting,
				CreatedAt:      time.Now(), // Will be asserted loosely
				UpdatedAt:      time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertInvoice: func(t *testing.T, expected, actual *models.Invoice) {
				assert.NotEqual(t, uuid.Nil, actual.ID) // Ensure ID was generated
				assert.Equal(t, expected.JobID, actual.JobID)
				assert.Equal(t, expected.IntervalNumber, actual.IntervalNumber)
				assert.Equal(t, expected.Value, actual.Value)
				assert.Equal(t, expected.State, actual.State)
				// Loosely assert time fields
				assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "Success_WithAdjustment",
			req: &dto.CreateInvoiceRequest{
				JobID:      uuid.New(),
				UserId:     uuid.New(),
				Adjustment: ptrFloat64(-25.50),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Rate:            100.0,
					Duration:        40,
					InvoiceInterval: 10,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 0,
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{
				req: nil, // Use gomock.Any()
				res: &models.Invoice{
					ID:             uuid.New(), // Simulate repo generating ID
					JobID:          uuid.Nil,   
					IntervalNumber: 1,          // Expected next interval
					Value:          (100.0 * 10) - 25.50, // Expected value with adjustment
					State:          models.InvoiceStateWaiting,
					CreatedAt:      time.Now(), // Simulate repo setting time
					UpdatedAt:      time.Now(), // Simulate repo setting time
				},
				err: nil,
			},
			expectedInvoice: &models.Invoice{
				ID:             uuid.Nil, 
				JobID:          uuid.Nil, 
				IntervalNumber: 1,
				Value:          (100.0 * 10) - 25.50,
				State:          models.InvoiceStateWaiting,
				CreatedAt:      time.Now(), // Will be asserted loosely
				UpdatedAt:      time.Now(), // Will be asserted loosely
			},
			expectedErr: nil,
			assertInvoice: func(t *testing.T, expected, actual *models.Invoice) {
				assert.NotEqual(t, uuid.Nil, actual.ID) // Ensure ID was generated
				assert.Equal(t, expected.JobID, actual.JobID)
				assert.Equal(t, expected.IntervalNumber, actual.IntervalNumber)
				assert.Equal(t, expected.Value, actual.Value)
				assert.Equal(t, expected.State, actual.State)
				// Loosely assert time fields
				assert.WithinDuration(t, expected.CreatedAt, actual.CreatedAt, time.Second)
				assert.WithinDuration(t, expected.UpdatedAt, actual.UpdatedAt, time.Second)
			},
		},
		{
			name: "Error_JobNotFound",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{}, // Not called
			mockInvoiceRepoCreate:               mockInvoiceRepoCreate{},               // Not called
			expectedInvoice:                     nil,
			expectedErr:                         services.ErrNotFound,
			assertInvoice:                       nil,
		},
		{
			name: "Error_JobRepoError",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{}, // Not called
			mockInvoiceRepoCreate:               mockInvoiceRepoCreate{},               // Not called
			expectedInvoice:                     nil,
			expectedErr:                         errors.New("internal error creating job: db error"), // Service wraps the error
			assertInvoice:                       nil,
		},
		{
			name: "Error_Forbidden_NotContractor",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(), // Different user
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					ContractorID: ptrUUID(uuid.New()), // Actual contractor
					State:        models.JobStateOngoing,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{}, // Not called
			mockInvoiceRepoCreate:               mockInvoiceRepoCreate{},               // Not called
			expectedInvoice:                     nil,
			expectedErr:                         services.ErrForbidden,
			assertInvoice:                       nil,
		},
		{
			name: "Error_Forbidden_NoContractor",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					ContractorID: nil, // No contractor
					State:        models.JobStateOngoing,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{}, // Not called
			mockInvoiceRepoCreate:               mockInvoiceRepoCreate{},               // Not called
			expectedInvoice:                     nil,
			expectedErr:                         services.ErrForbidden,
			assertInvoice:                       nil,
		},
		{
			name: "Error_InvalidState_JobNotOngoing",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					ContractorID: ptrUUID(uuid.Nil), 
					State:        models.JobStateWaiting, // Wrong state
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{}, // Not called
			mockInvoiceRepoCreate:               mockInvoiceRepoCreate{},               // Not called
			expectedInvoice:                     nil,
			expectedErr:                         services.ErrInvalidState,
			assertInvoice:                       nil,
		},
		{
			name: "Error_GetMaxIntervalRepoError",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:           uuid.Nil, 
					EmployerID:   uuid.New(),
					ContractorID: ptrUUID(uuid.Nil), 
					State:        models.JobStateOngoing,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 0,
				err: errors.New("db error"),
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{}, // Not called
			expectedInvoice:       nil,
			expectedErr:           errors.New("internal error creating job: db error"), // Service wraps the error
			assertInvoice:         nil,
		},
		{
			name: "Error_InvalidInvoiceInterval_ZeroJobInterval",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					InvoiceInterval: 0, // Invalid interval
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 0,
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{}, // Not called
			expectedInvoice:       nil,
			expectedErr:           services.ErrInvalidInvoiceInterval,
			assertInvoice:         nil,
		},
		{
			name: "Error_InvalidInvoiceInterval_ExceedsMax",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Duration:        20,
					InvoiceInterval: 10, // Max 2 intervals
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 2, // Already created 2 intervals
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{}, // Not called
			expectedInvoice:       nil,
			expectedErr:           services.ErrInvalidInvoiceInterval,
			assertInvoice:         nil,
		},
		{
			name: "Error_CreateRepoConflict",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Duration:        40,
					InvoiceInterval: 10,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 0,
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{
				req: nil, // Use gomock.Any()
				res: nil,
				err: storage.ErrConflict,
			},
			expectedInvoice: nil,
			expectedErr:     services.ErrConflict,
			assertInvoice:   nil,
		},
		{
			name: "Error_CreateRepoError",
			req: &dto.CreateInvoiceRequest{
				JobID:  uuid.New(),
				UserId: uuid.New(),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{
					ID:              uuid.Nil, 
					EmployerID:      uuid.New(),
					ContractorID:    ptrUUID(uuid.Nil), 
					State:           models.JobStateOngoing,
					Duration:        40,
					InvoiceInterval: 10,
				},
				err: nil,
			},
			mockInvoiceRepoGetMaxIntervalForJob: mockInvoiceRepoGetMaxIntervalForJob{
				req: &dto.GetMaxIntervalForJobRequest{JobID: uuid.Nil}, 
				res: 0,
				err: nil,
			},
			mockInvoiceRepoCreate: mockInvoiceRepoCreate{
				req: nil,
				res: nil,
				err: errors.New("db write error"),
			},
			expectedInvoice: nil,
			expectedErr:     errors.New("internal error creating job: db write error"), // Service wraps the error
			assertInvoice:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl := setupInvoiceServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			contractorID := uuid.New()
			requestingUserID := uuid.New() // Use a different ID for the forbidden case

			tt.req.JobID = jobID
			// Set UserId based on the test case
			if tt.name == "Error_Forbidden_NotContractor" || tt.name == "Error_Forbidden_NoContractor" {
				tt.req.UserId = requestingUserID // This user is NOT the contractor
			} else {
				tt.req.UserId = contractorID // Assume the requesting user IS the contractor for other cases
			}

			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.mockJobRepoGetByID.res.ContractorID != nil && tt.name != "Error_Forbidden_NoContractor" {
					*tt.mockJobRepoGetByID.res.ContractorID = contractorID
				}
			}
			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}

			if tt.mockInvoiceRepoGetMaxIntervalForJob.req != nil {
				tt.mockInvoiceRepoGetMaxIntervalForJob.req.JobID = jobID
			}

			if tt.mockInvoiceRepoCreate.res != nil {
				tt.mockInvoiceRepoCreate.res.JobID = jobID
				// Simulate repo setting times if not already set
				if tt.mockInvoiceRepoCreate.res.CreatedAt.IsZero() {
					tt.mockInvoiceRepoCreate.res.CreatedAt = time.Now()
				}
				if tt.mockInvoiceRepoCreate.res.UpdatedAt.IsZero() {
					tt.mockInvoiceRepoCreate.res.UpdatedAt = time.Now()
				}
			}
			if tt.expectedInvoice != nil {
				tt.expectedInvoice.JobID = jobID
				// Simulate repo setting times if not already set
				if tt.expectedInvoice.CreatedAt.IsZero() {
					tt.expectedInvoice.CreatedAt = time.Now()
				}
				if tt.expectedInvoice.UpdatedAt.IsZero() {
					tt.expectedInvoice.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockInvoiceRepoGetMaxIntervalForJob.req != nil {
				mockInvoiceRepo.EXPECT().GetMaxIntervalForJob(ctx, tt.mockInvoiceRepoGetMaxIntervalForJob.req).Return(tt.mockInvoiceRepoGetMaxIntervalForJob.res, tt.mockInvoiceRepoGetMaxIntervalForJob.err).Times(1)
			}
			if tt.mockInvoiceRepoCreate.res != nil || tt.mockInvoiceRepoCreate.err != nil {
				mockInvoiceRepo.EXPECT().Create(ctx, gomock.Any()).Return(tt.mockInvoiceRepoCreate.res, tt.mockInvoiceRepoCreate.err).Times(1)
			}

			// Call the service method
			invoice, err := invoiceService.CreateInvoice(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, invoice)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, invoice)
				if tt.assertInvoice != nil {
					tt.assertInvoice(t, tt.expectedInvoice, invoice)
				} else {
					assert.Equal(t, tt.expectedInvoice, invoice)
				}
			}
		})
	}
}

func TestInvoiceService_GetInvoiceByID(t *testing.T) {
	type mockInvoiceRepoGetByID struct {
		req *dto.GetInvoiceByIDRequest
		res *models.Invoice
		err error
	}
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}

	tests := []struct {
		name                   string
		req                    *dto.GetInvoiceByIDRequest
		mockInvoiceRepoGetByID mockInvoiceRepoGetByID
		mockJobRepoGetByID     mockJobRepoGetByID
		expectedInvoice        *models.Invoice
		expectedErr            error
	}{
		{
			name: "Success_AsEmployer",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.Nil, ContractorID: ptrUUID(uuid.New())}, 
				err: nil,
			},
			expectedInvoice: &models.Invoice{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
			expectedErr:     nil,
		},
		{
			name: "Success_AsContractor",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()}, // Requesting user is contractor
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			expectedInvoice: &models.Invoice{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
			expectedErr:     nil,
		},
		{
			name: "Error_InvoiceNotFound",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			expectedInvoice:    nil,
			expectedErr:        services.ErrNotFound,
		},
		{
			name: "Error_InvoiceRepoError",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			expectedInvoice:    nil,
			expectedErr:        errors.New("internal error getting invoice: db error"), // Service wraps the error
		},
		{
			name: "Error_JobRepoError",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New()}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			expectedInvoice: nil,
			expectedErr:     errors.New("internal error getting job: db error"), // Service wraps the error
		},
		{
			name: "Error_Forbidden",
			req:  &dto.GetInvoiceByIDRequest{ID: uuid.New(), UserId: uuid.New()}, // User not associated with the job
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New()}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), ContractorID: ptrUUID(uuid.New())}, 
				err: nil,
			},
			expectedInvoice: nil,
			expectedErr:     services.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl := setupInvoiceServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			invoiceID := uuid.New()
			jobID := uuid.New()
			employerID := uuid.New()
			contractorID := uuid.New()
			otherUserID := uuid.New()

			tt.req.ID = invoiceID
			if tt.name == "Success_AsEmployer" {
				tt.req.UserId = employerID
			} else if tt.name == "Success_AsContractor" {
				tt.req.UserId = contractorID
			} else if tt.name == "Error_Forbidden" {
				tt.req.UserId = otherUserID
			} else {
				tt.req.UserId = uuid.New() // Default for other error cases
			}

			if tt.mockInvoiceRepoGetByID.req != nil {
				tt.mockInvoiceRepoGetByID.req.ID = invoiceID
				tt.mockInvoiceRepoGetByID.req.UserId = tt.req.UserId // Match the request user ID
			}
			if tt.mockInvoiceRepoGetByID.res != nil {
				tt.mockInvoiceRepoGetByID.res.ID = invoiceID
				tt.mockInvoiceRepoGetByID.res.JobID = jobID
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				tt.mockJobRepoGetByID.res.EmployerID = employerID
				if tt.mockJobRepoGetByID.res.ContractorID != nil {
					*tt.mockJobRepoGetByID.res.ContractorID = contractorID
				}
			}

			if tt.expectedInvoice != nil {
				tt.expectedInvoice.ID = invoiceID
				tt.expectedInvoice.JobID = jobID
			}

			// Setup mocks
			if tt.mockInvoiceRepoGetByID.req != nil {
				mockInvoiceRepo.EXPECT().GetByID(ctx, tt.mockInvoiceRepoGetByID.req).Return(tt.mockInvoiceRepoGetByID.res, tt.mockInvoiceRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}

			// Call the service method
			invoice, err := invoiceService.GetInvoiceByID(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, invoice)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, invoice)
				assert.Equal(t, tt.expectedInvoice, invoice)
			}
		})
	}
}

func TestInvoiceService_UpdateInvoiceState(t *testing.T) {
	type mockInvoiceRepoGetByID struct {
		req *dto.GetInvoiceByIDRequest
		res *models.Invoice
		err error
	}
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockInvoiceRepoUpdateState struct {
		req *dto.UpdateInvoiceStateRequest
		res *models.Invoice
		err error
	}

	tests := []struct {
		name                       string
		req                        *dto.UpdateInvoiceStateRequest
		mockInvoiceRepoGetByID     mockInvoiceRepoGetByID
		mockJobRepoGetByID         mockJobRepoGetByID
		mockInvoiceRepoUpdateState mockInvoiceRepoUpdateState
		expectedInvoice            *models.Invoice
		expectedErr                error
	}{
		{
			name: "Success",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{
				req: &dto.UpdateInvoiceStateRequest{ID: uuid.Nil, NewState: models.InvoiceStateComplete, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateComplete, UpdatedAt: time.Now()}, 
				err: nil,
			},
			expectedInvoice: &models.Invoice{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateComplete, UpdatedAt: time.Now()}, 
			expectedErr:     nil,
		},
		{
			name: "Error_InvoiceNotFound",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{}, // Not called
			expectedInvoice:            nil,
			expectedErr:                services.ErrNotFound,
		},
		{
			name: "Error_InvoiceRepoError_Get",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{}, // Not called
			expectedInvoice:            nil,
			expectedErr:                errors.New("internal error getting invoice: db error"), // Service wraps the error
		},
		{
			name: "Error_JobRepoError",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{}, // Not called
			expectedInvoice:            nil,
			expectedErr:                errors.New("internal error getting job: db error"), // Service wraps the error
		},
		{
			name: "Error_Forbidden_NotContractor",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()}, // Different user
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.New())}, // Actual contractor
				err: nil,
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{}, // Not called
			expectedInvoice:            nil,
			expectedErr:                services.ErrForbidden,
		},
		{
			name: "Error_InvalidTransition",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateWaiting, UserId: uuid.New()}, // Invalid transition: Complete -> Waiting
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateComplete}, // Current state is Complete
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{}, // Not called
			expectedInvoice:            nil,
			expectedErr:                services.ErrInvalidTransition,
		},
		{
			name: "Error_UpdateRepoNotFound",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{
				req: &dto.UpdateInvoiceStateRequest{ID: uuid.Nil, NewState: models.InvoiceStateComplete, UserId: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound, // Repo returns NotFound on Update
			},
			expectedInvoice: nil,
			expectedErr:     services.ErrNotFound, // Service maps this
		},
		{
			name: "Error_UpdateRepoError",
			req:  &dto.UpdateInvoiceStateRequest{ID: uuid.New(), NewState: models.InvoiceStateComplete, UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoUpdateState: mockInvoiceRepoUpdateState{
				req: &dto.UpdateInvoiceStateRequest{ID: uuid.Nil, NewState: models.InvoiceStateComplete, UserId: uuid.Nil}, 
				res: nil,
				err: errors.New("db write error"),
			},
			expectedInvoice: nil,
			expectedErr:     errors.New("internal error updating invoice: db write error"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl := setupInvoiceServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			invoiceID := uuid.New()
			jobID := uuid.New()
			contractorID := uuid.New()
			actualContractorID := uuid.New()
			requestingUserID := uuid.New()

			tt.req.ID = invoiceID
			if tt.name == "Success" || tt.name == "Error_UpdateRepoNotFound" || tt.name == "Error_UpdateRepoError" {
				tt.req.UserId = contractorID
			} else if tt.name == "Error_Forbidden_NotContractor" {
				tt.req.UserId = requestingUserID
			} else if tt.name == "Error_InvalidTransition" {
				tt.req.UserId = contractorID // Ensure auth passes for this test
			} else {
				tt.req.UserId = uuid.New() // Default for other error cases
			}

			if tt.mockInvoiceRepoGetByID.req != nil {
				tt.mockInvoiceRepoGetByID.req.ID = invoiceID
			}
			if tt.mockInvoiceRepoGetByID.res != nil {
				tt.mockInvoiceRepoGetByID.res.ID = invoiceID
				tt.mockInvoiceRepoGetByID.res.JobID = jobID
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success" || tt.name == "Error_UpdateRepoNotFound" || tt.name == "Error_UpdateRepoError" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Error_Forbidden_NotContractor" {
					tt.mockJobRepoGetByID.res.ContractorID = &actualContractorID
				} else if tt.name == "Error_InvalidTransition" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID // Ensure auth passes for this test
				} else {
					tt.mockJobRepoGetByID.res.ContractorID = ptrUUID(uuid.New()) // Default for other cases
				}
			}

			if tt.mockInvoiceRepoUpdateState.req != nil {
				tt.mockInvoiceRepoUpdateState.req.ID = invoiceID
				tt.mockInvoiceRepoUpdateState.req.UserId = tt.req.UserId // Match the request user ID
			}
			if tt.mockInvoiceRepoUpdateState.res != nil {
				tt.mockInvoiceRepoUpdateState.res.ID = invoiceID
				tt.mockInvoiceRepoUpdateState.res.JobID = jobID
				// Simulate repo setting time if not already set
				if tt.mockInvoiceRepoUpdateState.res.UpdatedAt.IsZero() {
					tt.mockInvoiceRepoUpdateState.res.UpdatedAt = time.Now()
				}
			}

			if tt.expectedInvoice != nil {
				tt.expectedInvoice.ID = invoiceID
				tt.expectedInvoice.JobID = jobID
				// Simulate repo setting time if not already set
				if tt.expectedInvoice.UpdatedAt.IsZero() {
					tt.expectedInvoice.UpdatedAt = time.Now()
				}
			}

			// Setup mocks
			if tt.mockInvoiceRepoGetByID.req != nil {
				mockInvoiceRepo.EXPECT().GetByID(ctx, tt.mockInvoiceRepoGetByID.req).Return(tt.mockInvoiceRepoGetByID.res, tt.mockInvoiceRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockInvoiceRepoUpdateState.req != nil {
				mockInvoiceRepo.EXPECT().UpdateState(ctx, tt.mockInvoiceRepoUpdateState.req).Return(tt.mockInvoiceRepoUpdateState.res, tt.mockInvoiceRepoUpdateState.err).Times(1)
			}

			// Call the service method
			invoice, err := invoiceService.UpdateInvoiceState(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, invoice)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, invoice)
				// Loosely assert time fields
				assert.WithinDuration(t, tt.expectedInvoice.UpdatedAt, invoice.UpdatedAt, time.Second)
				// Compare other fields
				tt.expectedInvoice.UpdatedAt = invoice.UpdatedAt // Set expected to actual for comparison
				assert.Equal(t, tt.expectedInvoice, invoice)
			}
		})
	}
}

func TestInvoiceService_DeleteInvoice(t *testing.T) {
	type mockInvoiceRepoGetByID struct {
		req *dto.GetInvoiceByIDRequest
		res *models.Invoice
		err error
	}
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockInvoiceRepoDelete struct {
		req *dto.DeleteInvoiceRequest
		err error
	}

	tests := []struct {
		name                    string
		req                     *dto.DeleteInvoiceRequest
		mockInvoiceRepoGetByID  mockInvoiceRepoGetByID
		mockJobRepoGetByID      mockJobRepoGetByID
		mockInvoiceRepoDelete   mockInvoiceRepoDelete
		expectedErr             error
	}{
		{
			name: "Success",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{
				req: &dto.DeleteInvoiceRequest{ID: uuid.Nil}, 
				err: nil,
			},
			expectedErr: nil,
		},
		{
			name: "Error_InvoiceNotFound",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{}, // Not called
			expectedErr: services.ErrNotFound,
		},
		{
			name: "Error_InvoiceRepoError_Get",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockJobRepoGetByID: mockJobRepoGetByID{}, // Not called
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{}, // Not called
			expectedErr: errors.New("internal error getting invoice: db error"), // Service wraps the error
		},
		{
			name: "Error_JobRepoError",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{}, // Not called
			expectedErr: errors.New("internal error getting job: db error"), // Service wraps the error
		},
		{
			name: "Error_Forbidden_NotContractor",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()}, // Different user
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.New())}, // Actual contractor
				err: nil,
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{}, // Not called
			expectedErr: services.ErrForbidden,
		},
		{
			name: "Error_InvalidState_NotWaiting",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateComplete}, // Wrong state
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{}, // Not called
			expectedErr: services.ErrInvalidState,
		},
		{
			name: "Error_DeleteRepoNotFound",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{
				req: &dto.DeleteInvoiceRequest{ID: uuid.Nil}, 
				err: storage.ErrNotFound, // Repo returns NotFound on Delete
			},
			expectedErr: services.ErrNotFound, // Service maps this
		},
		{
			name: "Error_DeleteRepoError",
			req:  &dto.DeleteInvoiceRequest{ID: uuid.New(), UserId: uuid.New()},
			mockInvoiceRepoGetByID: mockInvoiceRepoGetByID{
				req: &dto.GetInvoiceByIDRequest{ID: uuid.Nil, UserId: uuid.Nil}, 
				res: &models.Invoice{ID: uuid.Nil, JobID: uuid.New(), State: models.InvoiceStateWaiting}, 
				err: nil,
			},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoDelete: mockInvoiceRepoDelete{
				req: &dto.DeleteInvoiceRequest{ID: uuid.Nil}, 
				err: errors.New("db delete error"),
			},
			expectedErr: errors.New("internal error deleting invoice: db delete error"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl := setupInvoiceServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			invoiceID := uuid.New()
			jobID := uuid.New()
			contractorID := uuid.New()
			actualContractorID := uuid.New()
			requestingUserID := uuid.New()

			tt.req.ID = invoiceID
			if tt.name == "Success" || tt.name == "Error_DeleteRepoNotFound" || tt.name == "Error_DeleteRepoError" {
				tt.req.UserId = contractorID
			} else if tt.name == "Error_Forbidden_NotContractor" {
				tt.req.UserId = requestingUserID
			} else if tt.name == "Error_InvalidState_NotWaiting" {
				tt.req.UserId = contractorID // Ensure auth passes for this test
			} else {
				tt.req.UserId = uuid.New() // Default for other error cases
			}

			if tt.mockInvoiceRepoGetByID.req != nil {
				tt.mockInvoiceRepoGetByID.req.ID = invoiceID
			}
			if tt.mockInvoiceRepoGetByID.res != nil {
				tt.mockInvoiceRepoGetByID.res.ID = invoiceID
				tt.mockInvoiceRepoGetByID.res.JobID = jobID
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success" || tt.name == "Error_DeleteRepoNotFound" || tt.name == "Error_DeleteRepoError" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Error_Forbidden_NotContractor" {
					tt.mockJobRepoGetByID.res.ContractorID = &actualContractorID
				} else if tt.name == "Error_InvalidState_NotWaiting" {
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID // Ensure auth passes for this test
				} else {
					tt.mockJobRepoGetByID.res.ContractorID = ptrUUID(uuid.New()) // Default for other cases
				}
			}

			if tt.mockInvoiceRepoDelete.req != nil {
				tt.mockInvoiceRepoDelete.req.ID = invoiceID
				// UserId is not part of the DeleteInvoiceRequest for the repo
			}

			// Setup mocks
			if tt.mockInvoiceRepoGetByID.req != nil {
				mockInvoiceRepo.EXPECT().GetByID(ctx, tt.mockInvoiceRepoGetByID.req).Return(tt.mockInvoiceRepoGetByID.res, tt.mockInvoiceRepoGetByID.err).Times(1)
			}
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockInvoiceRepoDelete.req != nil || tt.mockInvoiceRepoDelete.err != nil {
				mockInvoiceRepo.EXPECT().Delete(ctx, gomock.Any()).Return(tt.mockInvoiceRepoDelete.err).Times(1)
			}

			// Call the service method
			err := invoiceService.DeleteInvoice(ctx, tt.req)

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

func TestInvoiceService_ListInvoicesByJob(t *testing.T) {
	type mockJobRepoGetByID struct {
		req *dto.GetJobByIDRequest
		res *models.Job
		err error
	}
	type mockInvoiceRepoListByJob struct {
		req *dto.ListInvoicesByJobRequest
		res []models.Invoice
		err error
	}

	tests := []struct {
		name                       string
		req                        *dto.ListInvoicesByJobRequest
		mockJobRepoGetByID         mockJobRepoGetByID
		mockInvoiceRepoListByJob   mockInvoiceRepoListByJob
		expectedInvoices           []models.Invoice
		expectedErr                error
	}{
		{
			name: "Success_AsEmployer",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New(), Limit: 5, Offset: 0},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.Nil, ContractorID: ptrUUID(uuid.New())}, 
				err: nil,
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{
				req: &dto.ListInvoicesByJobRequest{JobID: uuid.Nil, UserId: uuid.Nil, Limit: 5, Offset: 0}, 
				res: []models.Invoice{
					{ID: uuid.New(), JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
					{ID: uuid.New(), JobID: uuid.Nil, State: models.InvoiceStateComplete}, 
				},
				err: nil,
			},
			expectedInvoices: []models.Invoice{
				{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
				{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateComplete}, 
			},
			expectedErr: nil,
		},
		{
			name: "Success_AsContractor",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New(), Limit: 5, Offset: 0}, // User is contractor
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), ContractorID: ptrUUID(uuid.Nil)}, 
				err: nil,
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{
				req: &dto.ListInvoicesByJobRequest{JobID: uuid.Nil, UserId: uuid.Nil, Limit: 5, Offset: 0}, 
				res: []models.Invoice{
					{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
				},
				err: nil,
			},
			expectedInvoices: []models.Invoice{
				{ID: uuid.Nil, JobID: uuid.Nil, State: models.InvoiceStateWaiting}, 
			},
			expectedErr: nil,
		},
		{
			name: "Error_JobNotFound",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: storage.ErrNotFound,
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{}, // Not called
			expectedInvoices:         nil,
			expectedErr:              services.ErrNotFound,
		},
		{
			name: "Error_JobRepoError",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: nil,
				err: errors.New("db error"),
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{}, // Not called
			expectedInvoices:         nil,
			expectedErr:              errors.New("internal error getting job: db error"), // Service wraps the error
		},
		{
			name: "Error_Forbidden",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New()}, // User not associated
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.New(), ContractorID: ptrUUID(uuid.New())}, 
				err: nil,
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{}, // Not called
			expectedInvoices:         nil,
			expectedErr:              services.ErrForbidden,
		},
		{
			name: "Error_ListRepoError",
			req:  &dto.ListInvoicesByJobRequest{JobID: uuid.New(), UserId: uuid.New()},
			mockJobRepoGetByID: mockJobRepoGetByID{
				req: &dto.GetJobByIDRequest{ID: uuid.Nil}, 
				res: &models.Job{ID: uuid.Nil, EmployerID: uuid.Nil}, 
				err: nil,
			},
			mockInvoiceRepoListByJob: mockInvoiceRepoListByJob{
				req: &dto.ListInvoicesByJobRequest{JobID: uuid.Nil, UserId: uuid.Nil}, 
				res: nil,
				err: errors.New("db list error"),
			},
			expectedInvoices: nil,
			expectedErr:     errors.New("internal error listing invoices: db list error"), // Service wraps the error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, invoiceService, mockInvoiceRepo, mockJobRepo, ctrl := setupInvoiceServiceTest(t)
			defer ctrl.Finish()

			// Set dynamic UUIDs
			jobID := uuid.New()
			employerID := uuid.New()
			contractorID := uuid.New()
			otherUserID := uuid.New()
			invoiceID := uuid.New()

			tt.req.JobID = jobID
			if tt.name == "Success_AsEmployer" || tt.name == "Error_ListRepoError" {
				tt.req.UserId = employerID
			} else if tt.name == "Success_AsContractor" {
				tt.req.UserId = contractorID
			} else if tt.name == "Error_Forbidden" {
				tt.req.UserId = otherUserID
			} else {
				tt.req.UserId = uuid.New() // Default for other error cases
			}

			if tt.mockJobRepoGetByID.req != nil {
				tt.mockJobRepoGetByID.req.ID = jobID
			}
			if tt.mockJobRepoGetByID.res != nil {
				tt.mockJobRepoGetByID.res.ID = jobID
				if tt.name == "Success_AsEmployer" || tt.name == "Error_ListRepoError" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Success_AsContractor" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else if tt.name == "Error_Forbidden" {
					tt.mockJobRepoGetByID.res.EmployerID = employerID
					tt.mockJobRepoGetByID.res.ContractorID = &contractorID
				} else {
					tt.mockJobRepoGetByID.res.EmployerID = uuid.New() // Default for other cases
					tt.mockJobRepoGetByID.res.ContractorID = ptrUUID(uuid.New()) // Default for other cases
				}
			}

			if tt.mockInvoiceRepoListByJob.req != nil {
				tt.mockInvoiceRepoListByJob.req.JobID = jobID
				tt.mockInvoiceRepoListByJob.req.UserId = tt.req.UserId // Match the request user ID
			}
			for i := range tt.mockInvoiceRepoListByJob.res {
				tt.mockInvoiceRepoListByJob.res[i].JobID = jobID
				tt.mockInvoiceRepoListByJob.res[i].ID = invoiceID
			}
			for i := range tt.expectedInvoices {
				tt.expectedInvoices[i].JobID = jobID
				tt.expectedInvoices[i].ID = invoiceID
			}

			// Setup mocks
			if tt.mockJobRepoGetByID.req != nil {
				mockJobRepo.EXPECT().GetByID(ctx, tt.mockJobRepoGetByID.req).Return(tt.mockJobRepoGetByID.res, tt.mockJobRepoGetByID.err).Times(1)
			}
			if tt.mockInvoiceRepoListByJob.req != nil {
				mockInvoiceRepo.EXPECT().ListByJob(ctx, tt.mockInvoiceRepoListByJob.req).Return(tt.mockInvoiceRepoListByJob.res, tt.mockInvoiceRepoListByJob.err).Times(1)
			}

			// Call the service method
			invoices, err := invoiceService.ListInvoicesByJob(ctx, tt.req)

			// Assert results
			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.expectedErr.Error() == err.Error() {
					assert.Equal(t, tt.expectedErr.Error(), err.Error())
				} else {
					assert.True(t, errors.Is(err, tt.expectedErr), "expected error %v, got %v", tt.expectedErr, err)
				}
				assert.Nil(t, invoices)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedInvoices, invoices)
			}
		})
	}
}
