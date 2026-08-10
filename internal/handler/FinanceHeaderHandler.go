package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetFinanceHeaders(c *gin.Context) {
	var headers []dto.FinanceHeader
	db := Connect()

	if err := db.Find(&headers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, headers)
}

func GetFinanceHeaderByID(c *gin.Context) {
	id := getID(c)
	var header dto.FinanceHeader
	db := Connect()

	if err := db.First(&header, "id = ?", id).Error; err != nil {
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

	// Same pattern as PostOrders/PostInvoice — Kas Bon IDs ("01/KB/26")
	// are also client-suggested sequential numbers (see Orders.go's
	// default tag), so they're exposed to the identical race.
	var conflict dto.FinanceHeader
	if err := db.Where("id = ?", header.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A finance header with this ID already exists"})
		return
	}

	if err := db.Create(&header).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A finance header with this ID already exists"})
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

	// BUG FIX: same issue as UpdateDelivery — binding the body straight
	// onto `existing` then calling Save() meant a client-supplied "id"
	// change silently updated ZERO rows (Save() built its WHERE clause
	// off the NEW id already sitting in the struct), while still
	// returning 200 OK. Rewritten to precheck the new id and update
	// anchored to the OLD id, same as UpdateOrders/UpdateDelivery.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.FinanceHeader
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	newId := existing.Id
	if _, ok := raw["id"]; ok && body.Id != "" {
		newId = body.Id
	}
	if newId != existing.Id {
		var conflict dto.FinanceHeader
		if err := db.Where("id = ?", newId).First(&conflict).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "A finance header with this ID already exists"})
			return
		}
	}

	updates := map[string]interface{}{"id": newId}
	if _, ok := raw["date"]; ok {
		updates["date"] = body.Date
	}
	if _, ok := raw["description"]; ok {
		updates["description"] = body.Description
	}

	// Anchored to the OLD id so production_item/operation_item rows (FK'd
	// on header_id) follow via ON UPDATE CASCADE, and so this actually
	// hits the right row regardless of whether id changed.
	if err := db.Model(&dto.FinanceHeader{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	var updated dto.FinanceHeader
	db.Where("id = ?", newId).First(&updated)
	c.JSON(http.StatusOK, updated)
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
