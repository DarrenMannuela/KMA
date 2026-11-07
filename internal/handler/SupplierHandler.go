package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSupplier(c *gin.Context) {
	var suppliers []dto.Supplier
	db := Connect()

	results := db.Find(&suppliers)
	if results.Error != nil {
		dbError(c, results.Error)
		return
	}
	c.JSON(http.StatusOK, suppliers)
}

func PostSupplier(c *gin.Context) {
	var supplier dto.Supplier
	db := Connect()

	if err := c.ShouldBindJSON(&supplier); err != nil {
		bindError(c, err)
		return
	}

	if err := db.Create(&supplier).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}

	c.JSON(http.StatusCreated, supplier)
}

func GetSupplierByID(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	var supplier dto.Supplier
	db := Connect()

	if err := db.Where("id = ?", id).First(&supplier).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		dbError(c, err)
		return
	}

	c.JSON(http.StatusOK, supplier)
}

// UpdateSupplier is a PATCH: only the fields sent change, and the id never
// does (the update is anchored to the loaded row).
func UpdateSupplier(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	var existing dto.Supplier
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
		return
	}

	// Which fields were sent; the body is cached, so binding twice is safe.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Supplier
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["supplier_name"]; ok {
		updates["supplier_name"] = body.SupplierName
	}
	if _, ok := raw["supplier_category"]; ok {
		updates["supplier_category"] = body.SupplierCategory
	}
	// No "id": a supplier's id isn't client-set.

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			dbError(c, err)
			return
		}
	}

	var updated dto.Supplier
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteSupplier removes a supplier with its catalogue and documents (their
// files stay on disk, so Undo can bring them back too).
func DeleteSupplier(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	deleteWithTrash(c, "supplier", id, "Supplier not found", func(tx *gorm.DB) (any, error) {
		snap, err := snapshotSupplier(tx, id)
		if err != nil {
			return nil, err
		}
		return snap, tx.Where("id = ?", id).Delete(&dto.Supplier{}).Error
	})
}
