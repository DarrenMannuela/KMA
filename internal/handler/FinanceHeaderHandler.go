package handler

import (
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetFinanceHeaders(c *gin.Context) {
	var headers []dto.FinanceHeader
	db := Connect()

	query := db.Preload("Supplier")
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}

	if err := query.Find(&headers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, headers)
}

func GetFinanceHeaderByID(c *gin.Context) {
	id := getID(c)
	var header dto.FinanceHeader
	db := Connect()

	if err := db.Preload("Supplier").First(&header, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Finance header not found"})
		return
	}

	c.JSON(http.StatusOK, header)
}

func PostFinanceHeader(c *gin.Context) {
	var header dto.FinanceHeader

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if header.Type != "production" && header.Type != "operation" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'production' or 'operation'"})
		return
	}

	if err := db.Create(&header).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}

	c.JSON(http.StatusCreated, header)
}

func UpdateFinanceHeader(c *gin.Context) {
	id := getID(c)
	db := Connect()

	var existing dto.FinanceHeader
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Finance header not found"})
		return
	}

	if err := c.ShouldBindBodyWithJSON(&existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if err := db.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	c.JSON(http.StatusOK, existing)
}

func DeleteFinanceHeader(c *gin.Context) {
	id := getID(c)
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.FinanceHeader{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Finance header not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
