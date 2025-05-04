package services

import (
	"errors"
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/storage"
	"log"
)

// isValidJobStateTransition defines the allowed state changes.
func isValidJobStateTransition(from, to models.JobState) bool {
	//Assign and Unassign already handle state changes (This validates all other transitions)
	switch from {
	case models.JobStateWaiting:
		return to == models.JobStateArchived
	case models.JobStateOngoing:
		// Can transition to Complete (by contractor/employer) or back to Waiting (if unassigned)
		return to == models.JobStateComplete
	case models.JobStateComplete:
		// Can transition to Archived (by employer)
		return to == models.JobStateArchived
	case models.JobStateArchived:
		// Terminal state
		return false
	default:
		return false
	}
}

// isValidInvoiceStateTransition checks if moving from current to next state is allowed.
func isValidInvoiceStateTransition(current, next models.InvoiceState) bool {
	switch current {
	case models.InvoiceStateWaiting:
		return next == models.InvoiceStateComplete
	case models.InvoiceStateComplete:
		return false // Cannot transition from Complete
	default:
		return false
	}
}

// mapRepoError maps storage errors to service errors
func mapRepoError(err error, operation string) error {
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