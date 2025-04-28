// internal/api/handlers/interfaces.go (or similar)
package handlers

import "github.com/gin-gonic/gin"

// UserHandlerInterface defines the methods needed by the user routes.
type UserHandlerInterface interface {
	GetUserByID(c *gin.Context)
	GetUsers(c *gin.Context)
	CreateUser(c *gin.Context)
	UpdateUser(c *gin.Context)
	DeleteUser(c *gin.Context)
}

// ItemHandlerInterface defines the methods needed by the item routes.
type ItemHandlerInterface interface {
	GetItems(c *gin.Context)
	CreateItem(c *gin.Context)
	GetItemByID(c *gin.Context)
	UpdateItem(c *gin.Context)
	DeleteItem(c *gin.Context)
}

// Ensure handlers implements the interface (compile-time check)
var _ UserHandlerInterface = (*UserHandler)(nil)
var _ ItemHandlerInterface = (*ItemHandler)(nil)