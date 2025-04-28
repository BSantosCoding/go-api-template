package dto

// CreateItemRequest defines the structure for creating a new item.
type CreateItemRequest struct {
	Name        string  `json:"name" validate:"required,min=2,max=100"`
	Description string  `json:"description" validate:"omitempty,max=500"`
	Price       float64 `json:"price" validate:"required,gt=0"` 
}

// UpdateItemRequest defines the structure for updating an existing item.
type UpdateItemRequest struct {
	Name        *string  `json:"name" validate:"omitempty,min=2,max=100"`
	Description *string  `json:"description" validate:"omitempty,max=500"`
	Price       *float64 `json:"price" validate:"omitempty,gt=0"`
}
