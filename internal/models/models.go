package models

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- Job State Enum ---
type JobState string

const (
	JobStateWaiting JobState = "Waiting"
	JobStateOngoing   JobState = "Ongoing"
	JobStateComplete  JobState = "Complete"
	JobStateArchived  JobState = "Archived"
)

// Scan implements the sql.Scanner interface for JobState
func (js *JobState) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		byteVal, ok := value.([]byte)
		if ok {
			strVal = string(byteVal)
		} else {
			return fmt.Errorf("failed to scan JobState: value is not string or []byte")
		}
	}
	v := JobState(strVal)
	switch v {
	case JobStateOngoing, JobStateComplete, JobStateArchived, JobStateWaiting:
		*js = v
		return nil
	default:
		return fmt.Errorf("invalid JobState value: %s", strVal)
	}
}

// Value implements the driver.Valuer interface for JobState
func (js JobState) Value() (driver.Value, error) {
	return string(js), nil
}

// --- Invoice State Enum ---
type InvoiceState string

const (
	InvoiceStateWaiting  InvoiceState = "Waiting"  // Waiting for employer action/payment
	InvoiceStateComplete InvoiceState = "Complete" // Paid or otherwise resolved
)

// Scan implements the sql.Scanner interface for InvoiceState
func (is *InvoiceState) Scan(value interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		byteVal, ok := value.([]byte)
		if ok {
			strVal = string(byteVal)
		} else {
			return fmt.Errorf("failed to scan InvoiceState: value is not string or []byte")
		}
	}
	v := InvoiceState(strVal)
	switch v {
	case InvoiceStateWaiting, InvoiceStateComplete:
		*is = v
		return nil
	default:
		return fmt.Errorf("invalid InvoiceState value: %s", strVal)
	}
}

// Value implements the driver.Valuer interface for InvoiceState
func (is InvoiceState) Value() (driver.Value, error) {
	return string(is), nil
}

// User represents a user in the system
type User struct {
	// Assuming 'id' in DB is UUID type
	ID uuid.UUID `json:"id" db:"id"` // Use uuid.UUID, add db tag if using sqlx/similar

	// Assuming 'name' in DB is VARCHAR/TEXT NOT NULL
	Name string `json:"name" db:"name"`

	// Assuming 'email' in DB is VARCHAR/TEXT UNIQUE NOT NULL
	Email string `json:"email" db:"email"`

	// Assuming 'password_hash' in DB is VARCHAR/TEXT NOT NULL
	PasswordHash string    `json:"-" db:"password_hash"`

	// Assuming 'created_at' in DB is TIMESTAMPTZ NOT NULL
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Assuming 'updated_at' in DB is TIMESTAMPTZ NOT NULL
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Job represents a work contract between an employer and a contractor.
type Job struct {
	ID              uuid.UUID  `json:"id" db:"id"`
	Rate            float64    `json:"rate" db:"rate"`
	Duration        int        `json:"duration" db:"duration"` // In hours (or define unit clearly)
	ContractorID    *uuid.UUID `json:"contractor_id,omitempty" db:"contractor_id"` // Pointer for NULLable UUID
	EmployerID      uuid.UUID  `json:"employer_id" db:"employer_id"`
	State           JobState   `json:"state" db:"state"`
	InvoiceInterval int        `json:"invoice_interval" db:"invoice_interval"` // In hours
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

// Invoice represents a bill generated for a Job based on the interval.
type Invoice struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	Value     float64      `json:"value" db:"value"`
	State     InvoiceState `json:"state" db:"state"`
	JobID     uuid.UUID    `json:"job_id" db:"job_id"`
	IntervalNumber int          `json:"interval_number" db:"interval_number"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

