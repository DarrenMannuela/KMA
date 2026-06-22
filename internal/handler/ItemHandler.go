package handler

import (
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetItems(c *gin.Context) {
	var items []dto.Items
	db := Connect()

	results := db.Find(&items)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
	}

	c.JSON(200, items)

}

func GetItemsByOrder(c *gin.Context) {
	id := c.Query("order_id")
	var items []dto.Items
	db := Connect()
	result := db.Where("order_id = ?", id).Find(&items)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, items)
}

func PostItems(c *gin.Context) {
	var newItems dto.Items
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	results := db.Create(&newItems)
	if results.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
	}
	c.JSON(201, newItems)

}

func UpdateItems(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	var updateItems dto.Items
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&updateItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
	}

	if err := db.First(&updateItems, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
	}
	db.Save(&updateItems)
	c.JSON(http.StatusOK, updateItems)
}

func DeleteItems(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Items{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
	}

	c.Status(http.StatusNoContent)
}
