package services

import "go-api-template/internal/models"

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
