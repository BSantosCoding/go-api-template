package services

import (
	"context"
	"go-api-template/internal/models"
	"go-api-template/internal/transport/dto"
)

//go:generate mockgen -source=interfaces.go -destination=../mocks/mock_services.go -package=mocks

// UserService defines the interface for user-related business logic.
type UserService interface {
	Register(ctx context.Context, req *dto.CreateUserRequest) (*models.User, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*models.User, string, string, error) // Returns user and token
	GetAll(ctx context.Context) ([]models.User, error)
	GetByID(ctx context.Context, req *dto.GetUserByIdRequest) (*models.User, error)
	GetByEmail(ctx context.Context, req *dto.GetUserByEmailRequest) (*models.User, error)
	Update(ctx context.Context, req *dto.UpdateUserRequest) (*models.User, error)
	Delete(ctx context.Context, req *dto.DeleteUserRequest) error
	Refresh(ctx context.Context, req *dto.RefreshRequest) (string, string, error)
	Logout(ctx context.Context, req *dto.LogoutRequest) error
}

// JobService defines the interface for job-related business logic.
type JobService interface {
	CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error)
	GetJobByID(ctx context.Context, req *dto.GetJobByIDRequest) (*models.Job, error)
	ListAvailableJobs(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]models.Job, error)
	ListJobsByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]models.Job, error)
	ListJobsByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]models.Job, error)
	UpdateJobDetails(ctx context.Context, req *dto.UpdateJobDetailsRequest) (*models.Job, error)
	UpdateJobState(ctx context.Context, req *dto.UpdateJobStateRequest) (*models.Job, error)
	DeleteJob(ctx context.Context, req *dto.DeleteJobRequest) error
}

// InvoiceService defines the interface for invoice-related business logic.
type InvoiceService interface {
	CreateInvoice(ctx context.Context, req *dto.CreateInvoiceRequest) (*models.Invoice, error)
	GetInvoiceByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*models.Invoice, error)
	UpdateInvoiceState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*models.Invoice, error)
	DeleteInvoice(ctx context.Context, req *dto.DeleteInvoiceRequest) error
	ListInvoicesByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]models.Invoice, error)
}

// JobApplicationService defines the interface for job application business logic.
type JobApplicationService interface {
	ApplyToJob(ctx context.Context, req *dto.ApplyToJobRequest) (*models.JobApplication, error)
	GetApplicationByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*models.JobApplication, error)
	ListApplicationsByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]models.JobApplication, error)
	ListApplicationsByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]models.JobApplication, error)
	AcceptApplication(ctx context.Context, req *dto.AcceptApplicationRequest) (*models.Job, error) // Returns the updated Job
	RejectApplication(ctx context.Context, req *dto.RejectApplicationRequest) (*models.JobApplication, error)
	WithdrawApplication(ctx context.Context, req *dto.WithdrawApplicationRequest) (*models.JobApplication, error)
}
