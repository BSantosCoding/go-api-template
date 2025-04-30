package storage

import "errors"

// --- Standard Storage Errors ---
var (
	ErrNotFound       = errors.New("resource not found")
	ErrConflict       = errors.New("resource conflict (e.g., duplicate unique field)") // General conflict
	ErrDuplicateEmail = errors.New("email address already exists") // Specific conflict for email
	// Add other custom errors as needed
)