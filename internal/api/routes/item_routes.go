package routes

import (
	"go-api-template/internal/api/handlers"
	"go-api-template/internal/storage" // Need storage interface

	"github.com/gin-gonic/gin"
)

// registerItemRoutes registers all routes related to items
func registerItemRoutes(rg *gin.RouterGroup, itemRepo storage.ItemRepository) {
	// Create the handler specific to this resource
	itemHandler := handlers.NewItemHandler(itemRepo)

	// Define the sub-group for items (e.g., /api/v1/items)
	items := rg.Group("/items")
	{
		items.GET("/", itemHandler.GetItems)
		items.POST("/", itemHandler.CreateItem)
		items.GET("/:id", itemHandler.GetItemByID)
		items.PUT("/:id", itemHandler.UpdateItem)
		items.DELETE("/:id", itemHandler.DeleteItem)
	}
}
