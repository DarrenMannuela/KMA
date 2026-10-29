package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetBuktiKasHeaders(c *gin.Context) {
	var headers []dto.BuktiKasHeader
	db := Connect()

	if err := db.Find(&headers).Error; err != nil {
		dbError(c, err)
		return
	}

	c.JSON(http.StatusOK, headers)
}

func GetBuktiKasHeaderByID(c *gin.Context) {
	id := getID(c)
	var header dto.BuktiKasHeader
	db := Connect()

	if err := db.First(&header, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bukti Kas header not found"})
		return
	}

	c.JSON(http.StatusOK, header)
}

func PostBuktiKasHeader(c *gin.Context) {
	var header dto.BuktiKasHeader

	db := Connect()
	if err := c.ShouldBindBodyWithJSON(&header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Same pattern as PostFinanceHeader — Bukti Kas IDs ("01/BK/KMA/26")
	// are also client-suggested sequential numbers, so they're exposed to
	// the identical race.
	var conflict dto.BuktiKasHeader
	if err := db.Where("id = ?", header.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A Bukti Kas header with this ID already exists"})
		return
	}

	if err := db.Create(&header).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A Bukti Kas header with this ID already exists"})
		return
	}

	c.JSON(http.StatusCreated, header)
}

func UpdateBuktiKasHeader(c *gin.Context) {
	id := getID(c)
	db := Connect()

	var existing dto.BuktiKasHeader
	if err := db.First(&existing, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bukti Kas header not found"})
		return
	}

	// Precheck a new id and update anchored to the old one (binding onto the
	// loaded row and calling Save() would update zero rows on a rename).
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.BuktiKasHeader
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	newId := existing.Id
	if _, ok := raw["id"]; ok && body.Id != "" {
		newId = body.Id
	}
	if newId != existing.Id {
		var conflict dto.BuktiKasHeader
		if err := db.Where("id = ?", newId).First(&conflict).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "A Bukti Kas header with this ID already exists"})
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

	// Anchored to the OLD id so bukti_kas_item rows (FK'd on header_id)
	// follow via ON UPDATE CASCADE, and so this actually hits the right
	// row regardless of whether id changed.
	if err := db.Model(&dto.BuktiKasHeader{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	var updated dto.BuktiKasHeader
	if err := db.Where("id = ?", newId).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteBuktiKasHeader(c *gin.Context) {
	id := getID(c)
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.BuktiKasHeader{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bukti Kas header not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
