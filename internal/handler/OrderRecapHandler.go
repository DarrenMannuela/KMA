package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetOrderRecap(c *gin.Context) {
	var orderRecap []dto.OrderRecap
	db := Connect()

	results := db.Find(&orderRecap)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, orderRecap)

}

func PostOrderRecap(c *gin.Context) {
	var newOrder dto.OrderRecap
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

func UpdateOrderRecap(c *gin.Context) {
	id := c.Param("id")
	var updateOrder dto.Items
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateOrder, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
	}
	db.Save(&updateOrder)
	c.JSON(http.StatusOK, updateOrder)
}

func DeleteOrderRecap(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Delete(&dto.OrderRecap{}, id)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order recap not found"})
	}

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	c.Status(http.StatusNoContent)
}
