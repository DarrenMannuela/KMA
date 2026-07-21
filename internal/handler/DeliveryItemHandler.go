package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetDeliveryItem(c *gin.Context) {
	var itemOrders []dto.DeliveryItem
	db := Connect()

	results := db.Find(&itemOrders)

	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, itemOrders)
}

func PostDeliveryItem(c *gin.Context) {
	var newDeliveryItems dto.DeliveryItem
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newDeliveryItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	results := db.Create(&newDeliveryItems)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}
	c.JSON(201, newDeliveryItems)
}

func UpdateDeliveryItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	// Load the existing row first — binding JSON into it afterwards means
	// only the fields present in the request body get overwritten, and
	// everything else keeps its current DB value (correct PATCH semantics).
	// Loading AFTER binding, as before, let db.First() silently discard
	// the incoming update since First() overwrites every field on the
	// struct with what's already in the DB.
	var existing dto.DeliveryItem
	if result := db.First(&existing, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item Order not found"})
		return
	}

	if err := c.ShouldBindBodyWithJSON(&existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if result := db.Save(&existing); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}
	c.JSON(http.StatusOK, existing)
}

func DeleteDeliveryItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.DeliveryItem{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item Order not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
