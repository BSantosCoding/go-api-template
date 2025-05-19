package services

import (
	"errors"
	"fmt"
	"go-api-template/ent"
	"go-api-template/ent/invoice"
	"go-api-template/ent/job"
	"go-api-template/internal/api"
	"go-api-template/internal/storage"
	"log"
)

// isValidJobStateTransition defines the allowed state changes.
func isValidJobStateTransition(from, to job.State) bool {
	//Assign and Unassign already handle state changes (This validates all other transitions)
	switch from {
	case job.StateWaiting:
		return to == job.StateArchived
	case job.StateOngoing:
		// Can transition to StateComplete (by contractor/employer) or back to Waiting (if unassigned)
		return to == job.StateComplete
	case job.StateComplete:
		// Can transition to StateArchived (by employer)
		return to == job.StateArchived
	case job.StateArchived:
		// Terminal state
		return false
	default:
		return false
	}
}

// isValidInvoiceStateTransition checks if moving from current to next state is allowed.
func isValidInvoiceStateTransition(current, next invoice.State) bool {
	switch current {
	case invoice.StateWaiting:
		return next == invoice.StateComplete
	case invoice.StateComplete:
		return false // Cannot transition from StateComplete
	default:
		return false
	}
}

// MapRepoError maps storage errors to service errors
func MapRepoError(err error, operation string) error {
	if errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("%w: %s", ErrNotFound, operation)
	}
	if errors.Is(err, storage.ErrConflict) {
		// The repo layer should provide more context for conflict errors if possible
		return fmt.Errorf("%w: %s (%v)", ErrConflict, operation, err)
	}
	if errors.Is(err, storage.ErrDuplicateEmail) { // Example specific conflict
		return fmt.Errorf("%w: %s (duplicate email)", ErrConflict, operation)
	}
	// Log other unexpected errors
	log.Printf("Unexpected repository error during %s: %v", operation, err)
	return fmt.Errorf("internal error during %s: %w", operation, err)
}

func ptrStr(s string) *string { return &s }

func ptrFloat64(f float64) *float64 { return &f }

func ptrFloat32(f float32) *float32 { return &f }

func ptrInt(i int) *int { return &i }

func MapEntUserToResponse(user *ent.User) api.DtoUserResponse {
	userResp := api.DtoUserResponse{
		Id:        ptrStr(user.ID.String()),
		Name:      &user.Name,
		Email:     &user.Email,
		CreatedAt: ptrStr(user.CreatedAt.String()),
		UpdatedAt: ptrStr(user.UpdatedAt.String()),
	}
	return userResp
}

func MapEntJobToResponse(job *ent.Job) api.DtoJobResponse {
	jobResp := api.DtoJobResponse{
		Id:              ptrStr(job.ID.String()),
		Rate:            ptrFloat32(float32(job.Rate)),
		CreatedAt:       ptrStr(job.CreatedAt.String()),
		UpdatedAt:       ptrStr(job.UpdatedAt.String()),
		Duration:        ptrInt(job.Duration),
		EmployerId:      ptrStr(job.EmployerID.String()),
		InvoiceInterval: ptrInt(job.InvoiceInterval),
		State:           ptrStr(job.State.String()),
		ContractorId:    ptrStr(job.ContractorID.String()),
	}
	return jobResp
}

func MapEntInvoiceToResponse(invoice *ent.Invoice) api.DtoInvoiceResponse {
	invoiceResp := api.DtoInvoiceResponse{
		Id:             ptrStr(invoice.ID.String()),
		Value:          ptrFloat32(float32(invoice.Value)),
		State:          ptrStr(invoice.State.String()),
		JobId:          ptrStr(invoice.JobID.String()),
		IntervalNumber: ptrInt(invoice.IntervalNumber),
		CreatedAt:      ptrStr(invoice.CreatedAt.String()),
		UpdatedAt:      ptrStr(invoice.UpdatedAt.String()),
	}
	return invoiceResp
}

func MapEntJobApplicationToResponse(application *ent.JobApplication) api.DtoJobApplicationResponse {
	applicationResp := api.DtoJobApplicationResponse{
		Id:           ptrStr(application.ID.String()),
		ContractorId: ptrStr(application.ContractorID.String()),
		JobId:        ptrStr(application.JobID.String()),
		State:        (*api.JobapplicationState)(ptrStr(application.State.String())),
		CreatedAt:    ptrStr(application.CreatedAt.String()),
		UpdatedAt:    ptrStr(application.UpdatedAt.String()),
	}
	return applicationResp
}
