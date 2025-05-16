package services

import (
	"errors"
	"fmt"
	"go-api-template/ent/invoice"
	"go-api-template/ent/job"
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
