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

	}

	results := db.Create(&newDeliveryItems)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newDeliveryItems)
}

func UpdateDeliveryItem(c *gin.Context) {
	id := c.Param("id")
	var UpdateDeliveryItem dto.DeliveryItem
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&UpdateDeliveryItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&UpdateDeliveryItem, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item Order not found"})
	}
	db.Save(&UpdateDeliveryItem)
	c.JSON(http.StatusOK, UpdateDeliveryItem)
}

func DeleteDeliveryItem(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.DeliveryItem{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item Order not found"})
	}

	c.Status(http.StatusNoContent)

}
