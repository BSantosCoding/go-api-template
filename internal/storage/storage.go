package storage

import (
	"context"
	"go-api-template/ent"
	"go-api-template/ent/jobapplication"
	"go-api-template/internal/transport/dto"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user data operations.
type UserRepository interface {
	GetAll(ctx context.Context) ([]*ent.User, error)
	GetByID(ctx context.Context, id *dto.GetUserByIdRequest) (*ent.User, error)
	GetByEmail(ctx context.Context, id *dto.GetUserByEmailRequest) (*ent.User, error)
	Create(ctx context.Context, user *dto.CreateUserRequest) (*ent.User, error) // Modify to return created user ID or full user if needed
	Update(ctx context.Context, user *dto.UpdateUserRequest) (*ent.User, error) // Modify to return updated user if needed
	Delete(ctx context.Context, id *dto.DeleteUserRequest) error
	WithTx(tx *ent.Tx) UserRepository
}

// JobRepository defines the interface for job data operations.
type JobRepository interface {
	Create(ctx context.Context, req *dto.CreateJobRequest) (*ent.Job, error)
	GetByID(ctx context.Context, req *dto.GetJobByIDRequest) (*ent.Job, error)
	ListAvailable(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]*ent.Job, error)
	ListByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]*ent.Job, error)
	ListByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]*ent.Job, error)
	Update(ctx context.Context, req *dto.UpdateJobRequest) (*ent.Job, error)
	Delete(ctx context.Context, req *dto.DeleteJobRequest) error
	WithTx(tx *ent.Tx) JobRepository
}

// InvoiceRepository defines the interface for invoice data operations.
type InvoiceRepository interface {
	Create(ctx context.Context, invoice *ent.Invoice) (*ent.Invoice, error)
	GetByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*ent.Invoice, error)
	ListByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]*ent.Invoice, error)
	UpdateState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*ent.Invoice, error)
	Delete(ctx context.Context, req *dto.DeleteInvoiceRequest) error
	GetMaxIntervalForJob(ctx context.Context, req *dto.GetMaxIntervalForJobRequest) (int, error)
	WithTx(tx *ent.Tx) InvoiceRepository
}

// JobApplicationRepository defines the interface for job application storage operations.
type JobApplicationRepository interface {
	Create(ctx context.Context, req *dto.CreateJobApplicationRequest) (*ent.JobApplication, error)
	GetByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*ent.JobApplication, error)
	ListByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]*ent.JobApplication, error)
	ListByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]*ent.JobApplication, error)
	GetByJobAndContractor(ctx context.Context, req *dto.GetByJobAndContractorRequest) ([]*ent.JobApplication, error)
	UpdateState(ctx context.Context, req *dto.UpdateJobApplicationStateRequest) (*ent.JobApplication, error)
	UpdateStateByJobID(ctx context.Context, jobID uuid.UUID, newState jobapplication.State, excludeApplicationID *uuid.UUID) error
	Delete(ctx context.Context, req *dto.DeleteJobApplicationRequest) error
	WithTx(tx *ent.Tx) JobApplicationRepository
}
