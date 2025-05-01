package services

import "errors"

// Define common service errors
var (
	ErrNotFound           = errors.New("resource not found")
	ErrForbidden          = errors.New("forbidden")
	ErrConflict           = errors.New("conflict") // e.g., duplicate email, state conflict
	ErrValidation         = errors.New("validation failed")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidState       = errors.New("invalid state for operation")
	ErrInvalidTransition  = errors.New("invalid state transition")
	ErrInvalidInvoiceInterval = errors.New("invalid invoice interval")
)