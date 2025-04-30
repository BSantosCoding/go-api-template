package storage

import (
	"context"
	"go-api-template/internal/models"
	"go-api-template/internal/transport/dto"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	GetAll(ctx context.Context) ([]models.User, error)
	GetByID(ctx context.Context, id *dto.GetUserByIdRequest) (*models.User, error)
	GetByEmail(ctx context.Context, id *dto.GetUserByEmailRequest) (*models.User, error)
	Create(ctx context.Context, user *dto.CreateUserRequest) (*models.User, error) // Modify to return created user ID or full user if needed
	Update(ctx context.Context, user *dto.UpdateUserRequest) (*models.User, error) // Modify to return updated user if needed
	Delete(ctx context.Context, id *dto.DeleteUserRequest) error
}

// JobRepository defines the interface for job data operations.
type JobRepository interface {
	Create(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error) 
	GetByID(ctx context.Context, req *dto.GetJobByIDRequest) (*models.Job, error)
	ListAvailable(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]models.Job, error)
	ListByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]models.Job, error)
	ListByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]models.Job, error)
	Update(ctx context.Context, req *dto.UpdateJobRequest) (*models.Job, error)
	Delete(ctx context.Context, req *dto.DeleteJobRequest) error
}

// InvoiceRepository defines the interface for invoice data operations.
type InvoiceRepository interface {
	Create(ctx context.Context, invoice *models.Invoice) (*models.Invoice, error)
	GetByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*models.Invoice, error)
	ListByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]models.Invoice, error)
	UpdateState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*models.Invoice, error)
	Delete(ctx context.Context, req *dto.DeleteInvoiceRequest) error
	GetMaxIntervalForJob(ctx context.Context, req *dto.GetMaxIntervalForJobRequest) (int, error)
}
