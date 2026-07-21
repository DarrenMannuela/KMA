package handler

import (
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetDelivery(c *gin.Context) {
	var deliveries []dto.Delivery
	db := Connect()

	results := db.Find(&deliveries)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}
	c.JSON(200, deliveries)
}

func GetDeliveryByID(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	var delivery dto.Delivery
	db := Connect()

	result := db.Where("id = ?", id).First(&delivery)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}
	c.JSON(http.StatusOK, delivery)
}

func PostDelivery(c *gin.Context) {
	var newDeliveries dto.Delivery
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newDeliveries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	results := db.Create(&newDeliveries)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}
	c.JSON(201, newDeliveries)

}

func UpdateDelivery(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	// Load first, then bind on top — see UpdateDeliveryItem for why the
	// old load-after-bind order silently discarded the update payload.
	var existing dto.Delivery
	if result := db.Where("id = ?", id).First(&existing); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
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

func DeleteDelivery(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Delivery{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
