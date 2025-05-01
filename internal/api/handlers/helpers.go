package handlers

import (
	"fmt"
	"go-api-template/internal/models"
	"go-api-template/internal/transport/dto"

	"github.com/go-playground/validator"
)

func FormatValidationErrors(err error) map[string]string {
	errorsMap := make(map[string]string)
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		errorsMap["error"] = "Invalid validation error type"
		return errorsMap
	}
	for _, fieldError := range validationErrors {
		fieldName := fieldError.Field()
		errorsMap[fieldName] = fmt.Sprintf("Field validation for '%s' failed on the '%s' tag", fieldName, fieldError.Tag())
		switch fieldError.Tag() {
		case "required":
			errorsMap[fieldName] = fmt.Sprintf("Field '%s' is required", fieldName)
		case "email":
			errorsMap[fieldName] = fmt.Sprintf("Field '%s' must be a valid email address", fieldName)
		case "min":
			errorsMap[fieldName] = fmt.Sprintf("Field '%s' must be at least %s characters long", fieldName, fieldError.Param())
		case "max":
			errorsMap[fieldName] = fmt.Sprintf("Field '%s' must be at most %s characters long", fieldName, fieldError.Param())
		case "uuid":
			errorsMap[fieldName] = fmt.Sprintf("Field '%s' must be a valid UUID", fieldName)
		}
	}
	return errorsMap
}

// MapUserModelToUserResponse converts a models.User to a dto.UserResponse
func MapUserModelToUserResponse(user *models.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

// MapJobModelToJobResponse converts a models.Job to a dto.JobResponse
func MapJobModelToJobResponse(job *models.Job) dto.JobResponse {
	// ... (implementation from previous step) ...
	resp := dto.JobResponse{
		ID:              job.ID,
		Rate:            job.Rate,
		Duration:        job.Duration,
		ContractorID:    job.ContractorID,
		EmployerID:      job.EmployerID,
		State:           string(job.State), // Convert enum to string
		InvoiceInterval: job.InvoiceInterval,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
	}
	return resp
}

// MapInvoiceModelToInvoiceResponse converts a models.Invoice to a dto.InvoiceResponse
func MapInvoiceModelToInvoiceResponse(invoice *models.Invoice) dto.InvoiceResponse {
	return dto.InvoiceResponse{
		ID:             invoice.ID,
		Value:          invoice.Value,
		State:          string(invoice.State), // Convert enum to string
		JobID:          invoice.JobID,
		IntervalNumber: invoice.IntervalNumber,
		CreatedAt:      invoice.CreatedAt,
		UpdatedAt:      invoice.UpdatedAt,
	}
}