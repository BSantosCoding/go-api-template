// /`home/bsant/`testing/go-api-template/internal/api/handlers/items.go
package handlers

import (
	"errors" // Import errors for checking specific storage errors
	"log"
	"net/http"

	"go-api-template/internal/models"
	"go-api-template/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
)

// ItemHandler holds the repository dependency for item operations
type ItemHandler struct {
	repo storage.ItemRepository
	validator *validator.Validate
}

// NewItemHandler creates a new ItemHandler with the given repository
func NewItemHandler(repo storage.ItemRepository, validate *validator.Validate) *ItemHandler {
	return &ItemHandler{repo: repo, validator: validate}
}

// GetItems godoc
// @Summary      List all items
// @Description  Retrieves a list of all available items.
// @Tags         items
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.Item "Successfully retrieved list of items"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /items [get]
func (h *ItemHandler) GetItems(c *gin.Context) {
	items, err := h.repo.GetAll(c.Request.Context()) // Use h.repo and pass context
	if err != nil {
		log.Printf("Error fetching items: %v", err) // Log the actual error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve items"})
		return
	}
	// Repository's GetAll should return an empty slice, not nil, if no items found
	c.JSON(http.StatusOK, items)
}

// GetItemByID godoc
// @Summary      Get an item by ID
// @Description  Retrieves details for a specific item by its ID.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Item ID" Format(uuid) // Specify path param
// @Success      200  {object}  models.Item "Successfully retrieved item"
// @Failure      404  {object}  map[string]string{error=string} "Item Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /items/{id} [get]
func (h *ItemHandler) GetItemByID(c *gin.Context) {
	id := c.Param("id") // Get ID from URL path

	item, err := h.repo.GetByID(c.Request.Context(), id) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		} else {
			log.Printf("Error fetching item by ID %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve item"})
		}
		return
	}
	c.JSON(http.StatusOK, item)
}

// CreateItem godoc
// @Summary      Create a new item
// @Description  Adds a new item to the database. ID can be optionally provided or will be generated.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        item body      models.Item true  "Item object to create" // Specify request body
// @Success      201  {object}  models.Item "Item created successfully"
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      409  {object}  map[string]string{error=string} "Conflict - Item already exists"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /items [post]
func (h *ItemHandler) CreateItem(c *gin.Context) {
	var newItem models.Item

	// Bind JSON request body to the newItem struct
	if err := c.ShouldBindJSON(&newItem); err != nil {
		log.Printf("Error binding item JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// --- ID Generation Strategy ---
	// Generate UUID if ID is not provided by the client
	if newItem.ID.String() == "" {
		newItem.ID = uuid.New()
	}

	// Validate required fields (example)
	if newItem.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item name is required"})
		return
	}
	if newItem.Price < 0 { // Example: Ensure price is not negative
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item price cannot be negative"})
		return
	}

	err := h.repo.Create(c.Request.Context(), &newItem) // Use h.repo
	if err != nil {
		// Assuming Create might return ErrConflict if ID is duplicated
		if errors.Is(err, storage.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Item with this ID already exists"})
		} else {
			log.Printf("Error creating item: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create item"})
		}
		return
	}

	// Return the created item
	c.JSON(http.StatusCreated, newItem)
}

// UpdateItem godoc
// @Summary      Update an existing item
// @Description  Updates details for an existing item identified by ID.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        id   path      string      true  "Item ID" Format(uuid)
// @Param        item body      models.Item true  "Item object with updated fields"
// @Success      200  {object}  models.Item "Item updated successfully" // Or 204 No Content if not returning body
// @Failure      400  {object}  map[string]string{error=string} "Bad Request - Invalid input"
// @Failure      404  {object}  map[string]string{error=string} "Item Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /items/{id} [put]
func (h *ItemHandler) UpdateItem(c *gin.Context) {
	id := c.Param("id")
	var itemUpdates models.Item

	// Set the ID from the path parameter for the response object
	parsedId, err := uuid.Parse(id)
	if err != nil {
		log.Printf("Error parsing ID %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Id parsing error: " + err.Error()})
	}
	itemUpdates.ID = parsedId

	if err := c.ShouldBindJSON(&itemUpdates); err != nil {
		log.Printf("Error binding item update JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Basic validation
	if itemUpdates.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item name is required"})
		return
	}
	if itemUpdates.Price < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Item price cannot be negative"})
		return
	}

	// The repository Update method uses the 'id' from the URL parameter.
	err = h.repo.Update(c.Request.Context(), id, &itemUpdates) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		} else {
			// Note: Update might also return ErrConflict if constraints are violated
			log.Printf("Error updating item %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item"})
		}
		return
	}

	// Return the potentially updated item representation (or fetch it again if needed)
	c.JSON(http.StatusOK, itemUpdates)
}

// DeleteItem godoc
// @Summary      Delete an item by ID
// @Description  Removes an item from the database by its ID.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Item ID" Format(uuid)
// @Success      204  {object}  nil "Item deleted successfully" // 204 No Content
// @Failure      404  {object}  map[string]string{error=string} "Item Not Found"
// @Failure      500  {object}  map[string]string{error=string} "Internal Server Error"
// @Router       /items/{id} [delete]
func (h *ItemHandler) DeleteItem(c *gin.Context) {
	id := c.Param("id")

	err := h.repo.Delete(c.Request.Context(), id) // Use h.repo
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		} else {
			log.Printf("Error deleting item %s: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		}
		return
	}

	c.Status(http.StatusNoContent) // Standard response for successful DELETE
}
