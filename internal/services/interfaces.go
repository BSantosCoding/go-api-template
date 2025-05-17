package services

import (
	"context"
	"go-api-template/ent"
	"go-api-template/internal/transport/dto"
)

// UserService defines the interface for user-related business logic.
type UserService interface {
	Register(ctx context.Context, req *dto.CreateUserRequest) (*ent.User, error)
	Login(ctx context.Context, req *dto.LoginRequest) (*ent.User, string, string, error) // Returns user and token
	GetAll(ctx context.Context) ([]*ent.User, error)
	GetByID(ctx context.Context, req *dto.GetUserByIdRequest) (*ent.User, error)
	GetByEmail(ctx context.Context, req *dto.GetUserByEmailRequest) (*ent.User, error)
	Update(ctx context.Context, req *dto.UpdateUserRequest) (*ent.User, error)
	Delete(ctx context.Context, req *dto.DeleteUserRequest) error
	Refresh(ctx context.Context, req *dto.RefreshRequest) (string, string, error)
	Logout(ctx context.Context, req *dto.LogoutRequest) error
}

// JobService defines the interface for job-related business logic.
type JobService interface {
	CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*ent.Job, error)
	GetJobByID(ctx context.Context, req *dto.GetJobByIDRequest) (*ent.Job, error)
	ListAvailableJobs(ctx context.Context, req *dto.ListAvailableJobsRequest) ([]*ent.Job, error)
	ListJobsByEmployer(ctx context.Context, req *dto.ListJobsByEmployerRequest) ([]*ent.Job, error)
	ListJobsByContractor(ctx context.Context, req *dto.ListJobsByContractorRequest) ([]*ent.Job, error)
	UpdateJobDetails(ctx context.Context, req *dto.UpdateJobDetailsRequest) (*ent.Job, error)
	UpdateJobState(ctx context.Context, req *dto.UpdateJobStateRequest) (*ent.Job, error)
	DeleteJob(ctx context.Context, req *dto.DeleteJobRequest) error
}

// InvoiceService defines the interface for invoice-related business logic.
type InvoiceService interface {
	CreateInvoice(ctx context.Context, req *dto.CreateInvoiceRequest) (*ent.Invoice, error)
	GetInvoiceByID(ctx context.Context, req *dto.GetInvoiceByIDRequest) (*ent.Invoice, error)
	UpdateInvoiceState(ctx context.Context, req *dto.UpdateInvoiceStateRequest) (*ent.Invoice, error)
	DeleteInvoice(ctx context.Context, req *dto.DeleteInvoiceRequest) error
	ListInvoicesByJob(ctx context.Context, req *dto.ListInvoicesByJobRequest) ([]*ent.Invoice, error)
}

// JobApplicationService defines the interface for job application business logic.
type JobApplicationService interface {
	ApplyToJob(ctx context.Context, req *dto.ApplyToJobRequest) (*ent.JobApplication, error)
	GetApplicationByID(ctx context.Context, req *dto.GetJobApplicationByIDRequest) (*ent.JobApplication, error)
	ListApplicationsByContractor(ctx context.Context, req *dto.ListJobApplicationsByContractorRequest) ([]*ent.JobApplication, error)
	ListApplicationsByJob(ctx context.Context, req *dto.ListJobApplicationsByJobRequest) ([]*ent.JobApplication, error)
	AcceptApplication(ctx context.Context, req *dto.AcceptApplicationRequest) (*ent.Job, error) // Returns the updated Job
	RejectApplication(ctx context.Context, req *dto.RejectApplicationRequest) (*ent.JobApplication, error)
	WithdrawApplication(ctx context.Context, req *dto.WithdrawApplicationRequest) (*ent.JobApplication, error)
}
