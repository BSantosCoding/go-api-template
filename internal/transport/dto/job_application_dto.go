package dto

import (
	"go-api-template/internal/models"

	"github.com/google/uuid"
)

// CreateJobApplicationRequest is used internally by the ApplyToJob service method.
type CreateJobApplicationRequest struct {
	JobID        uuid.UUID `json:"job_id"`      // Provided by the user
	ContractorID uuid.UUID `json:"contractor_id"` // Set from user context
}

type JobApplicationResponse struct {
	ID           uuid.UUID                `json:"id"`
	ContractorID uuid.UUID                `json:"contractor_id"`
	JobID        uuid.UUID                `json:"job_id"`
	State        models.JobApplicationState `json:"state"`
	CreatedAt    string                   `json:"created_at"`
	UpdatedAt    string                   `json:"updated_at"`
}

type GetJobApplicationByIDRequest struct {
	ID     uuid.UUID `json:"-" validate:"required"` // From path
	UserID uuid.UUID `json:"-"`                          // Set from user context for auth check
}

// ListJobApplicationsByContractorRequest defines parameters for listing applications by contractor.
type ListJobApplicationsByContractorRequest struct {
	ContractorID uuid.UUID `json:"-" validate:"required"` // Set from user context
	Limit        int       `form:"limit,default=10" validate:"omitempty,gte=0"`
	Offset       int       `form:"offset,default=0" validate:"omitempty,gte=0"`
}

// ListJobApplicationsByJobRequest defines parameters for listing applications by job.
type ListJobApplicationsByJobRequest struct {
	JobID  uuid.UUID `json:"-" validate:"required"` // From path
	UserID uuid.UUID `json:"-"`                          // Set from user context for auth check
	Limit        int       `form:"limit,default=10" validate:"omitempty,gte=0"`
	Offset       int       `form:"offset,default=0" validate:"omitempty,gte=0"`
}

type UpdateJobApplicationStateRequest struct {
	ID    uuid.UUID                `json:"-" validate:"required"` // From path
	State models.JobApplicationState `json:"state" validate:"required,job_application_state"`
}

type DeleteJobApplicationRequest struct {
	ID     uuid.UUID `json:"-" validate:"required"` // From path
	UserID uuid.UUID `json:"-"`                          // Set from user context for auth check
}

type ApplyToJobRequest struct {
	JobID        uuid.UUID `json:"job_id" validate:"required"` // Job ID to apply for (from request body or path)
	ContractorID uuid.UUID `json:"-"`                               // Set from user context
}

type AcceptApplicationRequest struct {
	ApplicationID uuid.UUID `json:"-" validate:"required"` // From path
	UserID        uuid.UUID `json:"-"`                          // Set from user context (must be employer)
}

type RejectApplicationRequest struct {
	ApplicationID uuid.UUID `json:"-" validate:"required"` // From path
	UserID        uuid.UUID `json:"-"`                          // Set from user context (employer or applicant)
}

type WithdrawApplicationRequest struct {
	ApplicationID uuid.UUID `json:"-" validate:"required"` // From path
	UserID        uuid.UUID `json:"-"`                          // Set from user context (must be applicant)
}