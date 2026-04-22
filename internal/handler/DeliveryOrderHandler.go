package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetDeliveryOrder(c *gin.Context) {
	var deliveryOrders []dto.DeliveryOrder
	db := Connect()

	results := db.Find(&deliveryOrders)

	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, deliveryOrders)
}

func PostDeliveryOrder(c *gin.Context) {
	var newDeliveryOrders dto.DeliveryOrder
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newDeliveryOrders); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})

	}

	results := db.Create(&newDeliveryOrders)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newDeliveryOrders)
}

func UpdateDeliveryOrder(c *gin.Context) {
	id := c.Param("id")
	var UpdateDeliveryOrder dto.DeliveryOrder
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&UpdateDeliveryOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&UpdateDeliveryOrder, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "DeliveryOrder not found"})
	}
	db.Save(&UpdateDeliveryOrder)
	c.JSON(http.StatusOK, UpdateDeliveryOrder)
}

func DeleteDeliveryOrder(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Delete(&dto.DeliveryOrder{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "DeliverOrder not found"})
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	c.Status(http.StatusNoContent)

}
