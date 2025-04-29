// internal/transport/dto/job_dto.go
package dto

import (
	"go-api-template/internal/models" // Import models for enums
	"time"

	"github.com/google/uuid"
)

// --- Job Request DTOs ---

// CreateJobRequest defines the structure for creating a new job posting.
type CreateJobRequest struct {
	Rate            float64 `json:"rate" validate:"required,gt=0"`              // Rate per hour, must be positive
	Duration        int     `json:"duration" validate:"required,gt=0"`          // Duration in hours, must be positive
	InvoiceInterval int     `json:"invoice_interval" validate:"required,gt=0"` // Interval in hours, must be positive
	EmployerID      uuid.UUID `json:"-"` // Set internally by handler from auth context
}

// GetJobByIDRequest defines the structure for getting a job by ID.
type GetJobByIDRequest struct {
	ID uuid.UUID `json:"-" validate:"required,uuid"`
}

// ListAvailableJobsRequest defines parameters for listing available jobs.
type ListAvailableJobsRequest struct {
	Limit   int      `form:"limit,default=10"`
	Offset  int      `form:"offset,default=0"`
	MinRate *float64 `form:"min_rate" validate:"omitempty,gt=0"` 
	MaxRate *float64 `form:"max_rate" validate:"omitempty,gt=0,gtefield=MinRate"`
}

// ListJobsByEmployerRequest defines parameters for listing jobs by employer.
type ListJobsByEmployerRequest struct {
	EmployerID uuid.UUID        `json:"-" validate:"required,uuid"` // Set internally by handler
	Limit      int              `form:"limit,default=10"`
	Offset     int              `form:"offset,default=0"`
	State      *models.JobState `form:"state" validate:"omitempty,oneof=Waiting Ongoing Complete Archived"` 
	MinRate    *float64         `form:"min_rate" validate:"omitempty,gt=0"`                         
	MaxRate    *float64         `form:"max_rate" validate:"omitempty,gt=0,gtefield=MinRate"`        
}

// ListJobsByContractorRequest defines parameters for listing jobs by contractor.
type ListJobsByContractorRequest struct {
	ContractorID uuid.UUID        `json:"-" validate:"required,uuid"` // Set internally by handler
	Limit        int              `form:"limit,default=10"`
	Offset       int              `form:"offset,default=0"`
	State        *models.JobState `form:"state" validate:"omitempty,oneof=Waiting Ongoing Complete Archived"` 
	MinRate      *float64         `form:"min_rate" validate:"omitempty,gt=0"`                         
	MaxRate      *float64         `form:"max_rate" validate:"omitempty,gt=0,gtefield=MinRate"`        
}

// UpdateJobRequest defines the structure for updating a job.
// Different updates might need different DTOs (e.g., AssignContractor, UpdateJobState).
// This is a general example; refine based on allowed updates.
type UpdateJobRequest struct {
	ID           uuid.UUID        `json:"-" validate:"required,uuid"` // From URL path
	Rate         *float64         `json:"rate,omitempty" validate:"omitempty,gt=0"`
	Duration     *int             `json:"duration,omitempty" validate:"omitempty,gt=0"`
	ContractorID *uuid.UUID       `json:"contractor_id,omitempty" validate:"omitempty,uuid"` // For assigning/unassigning
	State        *models.JobState `json:"state,omitempty" validate:"omitempty,oneof=Waiting Ongoing Complete Archived"`
	// InvoiceInterval might not be updatable after creation
}

// UpdateJobDetailsRequest defines the structure for updating rate/duration.
type UpdateJobDetailsRequest struct {
	Rate     *float64 `json:"rate,omitempty" validate:"omitempty,gt=0"`
	Duration *int     `json:"duration,omitempty" validate:"omitempty,gt=0"`
}

// AssignContractorRequest defines the structure for assigning a contractor.
type AssignContractorRequest struct {
	ContractorID uuid.UUID `json:"contractor_id" validate:"required,uuid"`
}

// UpdateJobStateRequest defines the structure for updating the job state.
type UpdateJobStateRequest struct {
	State models.JobState `json:"state" validate:"required,oneof=Waiting Ongoing Complete Archived"`
}

// DeleteJobRequest defines the structure for deleting a job.
type DeleteJobRequest struct {
	ID uuid.UUID `json:"-" validate:"required,uuid"`
}

// JobResponse defines the standard job data returned to the client.
type JobResponse struct {
	ID              uuid.UUID  `json:"id"`
	Rate            float64    `json:"rate"`
	Duration        int        `json:"duration"`
	ContractorID    *uuid.UUID `json:"contractor_id,omitempty"`
	EmployerID      uuid.UUID  `json:"employer_id"`
	State           string     `json:"state"`
	InvoiceInterval int        `json:"invoice_interval"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// Consider adding Employer/Contractor details (names/emails) if needed
}

