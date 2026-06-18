package handler

import (
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetOrders(c *gin.Context) {
	var orders []dto.Orders
	db := Connect()

	results := db.Find(&orders)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, orders)

}

func PostOrders(c *gin.Context) {
	var newOrder dto.Orders
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	results := db.Create(&newOrder)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newOrder)
}

func UpdateOrders(c *gin.Context) {
	id := getID(c)
	var existing dto.Orders
	db := Connect()

	// Find existing record first
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Bind update fields
	var body dto.Orders
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	body.Id = existing.Id // keep original ID
	if err := db.Save(&body).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, body)
}

func DeleteOrders(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Orders{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Production not found"})
	}

	c.Status(http.StatusNoContent)
}
