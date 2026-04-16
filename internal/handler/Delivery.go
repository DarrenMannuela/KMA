package handler

import (
	"net/http"

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

func PostDelivery(c *gin.Context) {
	var newDeliveries dto.Delivery
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newDeliveries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	results := db.Create(&newDeliveries)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newDeliveries)

}

func UpdateDelivery(c *gin.Context) {
	id := c.Param("id")
	var updateDelivery dto.Delivery
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateDelivery); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateDelivery, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
	}
	db.Save(&updateDelivery)
	c.JSON(http.StatusOK, updateDelivery)
}

func DeleteDelivery(c *gin.Context) {
	id := c.Param("id")

}
