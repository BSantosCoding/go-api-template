// internal/transport/dto/invoice_dto.go
package dto

import (
	"go-api-template/internal/models" // Import models for enums
	"time"

	"github.com/google/uuid"
)

// CreateInvoiceRequest defines the structure for creating a new invoice.
// Value and IntervalNumber might be calculated by the handler/service layer.
// JobID might come from the URL path or context.
type CreateInvoiceRequest struct {
	JobID          uuid.UUID `json:"job_id" validate:"required,uuid"`
	Adjustment *float64  `json:"adjustment,omitempty" validate:"omitempty"`
	UserId uuid.UUID `json:"-"`
}

// GetInvoiceByIDRequest defines the structure for getting an invoice by ID.
type GetInvoiceByIDRequest struct {
	ID uuid.UUID `json:"-" validate:"required,uuid"`
	UserId uuid.UUID `json:"-"`
}

// ListInvoicesByJobRequest defines parameters for listing invoices for a specific job.
type ListInvoicesByJobRequest struct {
	JobID  uuid.UUID            `json:"-" validate:"required,uuid"` // From URL path
	Limit  int                  `form:"limit,default=10"`
	Offset int                  `form:"offset,default=0"`
	State  *models.InvoiceState `form:"state" validate:"omitempty,oneof=Waiting Complete"`
	UserId uuid.UUID `json:"-"`
}

// UpdateInvoiceStateRequest defines the structure for updating an invoice's state.
// ID usually comes from the URL path.
type UpdateInvoiceStateRequest struct {
	ID       uuid.UUID           `json:"-" validate:"required,uuid"` // From URL path
	NewState models.InvoiceState `json:"state" validate:"required,oneof=Waiting Complete"`
	UserId uuid.UUID `json:"-"`
}

// DeleteInvoiceRequest defines the structure for deleting an invoice.
type DeleteInvoiceRequest struct {
	ID uuid.UUID `json:"-" validate:"required,uuid"`
	UserId uuid.UUID `json:"-"`
}

// GetMaxIntervalForJobRequest defines the structure for getting the max interval.
type GetMaxIntervalForJobRequest struct {
	JobID uuid.UUID `validate:"required,uuid"` // JobID is the input needed
}

// InvoiceResponse defines the standard invoice data returned to the client.
type InvoiceResponse struct {
	ID             uuid.UUID `json:"id"`
	Value          float64   `json:"value"`
	State          string    `json:"state"` // Return state as string
	JobID          uuid.UUID `json:"job_id"`
	IntervalNumber int       `json:"interval_number"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

