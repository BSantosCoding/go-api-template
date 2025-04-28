package handlers

import "github.com/go-playground/validator"

func formatValidationErrors(errs validator.ValidationErrors) map[string]string {
    errors := make(map[string]string)
    for _, err := range errs {
        // Customize error messages based on tag and field
        errors[err.Field()] = "Field validation for '" + err.Field() + "' failed on the '" + err.Tag() + "' tag"
        // switch err.Tag() {
        // case "required":
        // 	errors[err.Field()] = err.Field() + " is required"
        // case "email":
        //  errors[err.Field()] = err.Field() + " must be a valid email address"
        // default:
        // 	errors[err.Field()] = "Field validation for '" + err.Field() + "' failed on the '" + err.Tag() + "' tag"
        // }
    }
    return errors
}