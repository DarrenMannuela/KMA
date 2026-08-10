package handler

import (
	"net/http"
	"strconv"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetProductionItems(c *gin.Context) {
	var items []dto.ProductionItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

// GetProductionItemsByHeader returns items for a single Kas Bon.
func GetProductionItemsByHeader(c *gin.Context) {
	headerId := c.Query("header_id")
	if headerId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "header_id is required"})
		return
	}

	var items []dto.ProductionItem
	db := Connect()

	if err := db.Where("header_id = ?", headerId).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func GetProductionItemsGrouped(c *gin.Context) {
	var items []dto.ProductionItem
	db := Connect()

	if err := db.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	grouped := make(map[string][]dto.ProductionItem)
	for _, item := range items {
		grouped[item.HeaderId] = append(grouped[item.HeaderId], item)
	}

	c.JSON(http.StatusOK, grouped)
}

func PostProductionItem(c *gin.Context) {
	var item dto.ProductionItem

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

func UpdateProductionItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	var existing dto.ProductionItem
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Production item not found"})
		return
	}
	originalId := existing.Id

	if err := c.ShouldBindBodyWithJSON(&existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	// Pin the PK back after binding — see UpdateDeliveryItem for why:
	// a client-sent "id" would otherwise redirect Save()'s WHERE clause
	// and silently no-op instead of updating this row.
	existing.Id = originalId

	if err := db.Save(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	c.JSON(http.StatusOK, existing)
}

func DeleteProductionItem(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item id"})
		return
	}

	db := Connect()
	result := db.Where("id = ?", id).Delete(&dto.ProductionItem{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Production item not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
