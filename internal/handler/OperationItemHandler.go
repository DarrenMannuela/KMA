package handler

import (
	"net/http"
	"strconv"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetOperationItems(c *gin.Context) {
	var items []dto.OperationItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func GetOperationItemsByHeader(c *gin.Context) {
	headerId := c.Query("header_id")
	if headerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "header_id is required"})
		return
	}

	var items []dto.OperationItem
	db := Connect()

	if err := db.Where("header_id = ?", headerId).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func GetOperationItemsGrouped(c *gin.Context) {
	var items []dto.OperationItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	grouped := make(map[string][]dto.OperationItem)
	for _, item := range items {
		grouped[item.HeaderId] = append(grouped[item.HeaderId], item)
	}

	c.JSON(http.StatusOK, grouped)
}

func PostOperationItem(c *gin.Context) {
	var item dto.OperationItem

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if item.HeaderId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "header_id is required"})
		return
	}

	if err := db.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}

	c.JSON(http.StatusCreated, item)
}

func UpdateOperationItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	var existing dto.OperationItem
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation item not found"})
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

func DeleteOperationItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	result := db.Where("id = ?", id).Delete(&dto.OperationItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Operation item not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
