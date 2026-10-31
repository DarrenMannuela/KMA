package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetSupplierMaterialColors(c *gin.Context) {
	var items []dto.SupplierMaterialColor
	db := Connect()

	if result := db.Find(&items); result.Error != nil {
		dbError(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetSupplierMaterialColorsByMaterial(c *gin.Context) {
	materialId := c.Query("supplier_material_id")
	var items []dto.SupplierMaterialColor
	db := Connect()

	if result := db.Where("supplier_material_id = ?", materialId).Find(&items); result.Error != nil {
		dbError(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, items)
}

// GetSupplierMaterialColorsGrouped returns every colour, grouped by its
// material id — the picker loads a supplier's whole colour set in one
// request instead of one round trip per material, same shape as
// GetClientItemPricesGrouped.
func GetSupplierMaterialColorsGrouped(c *gin.Context) {
	var items []dto.SupplierMaterialColor
	db := Connect()

	if result := db.Find(&items); result.Error != nil {
		dbError(c, result.Error)
		return
	}
	grouped := map[string][]dto.SupplierMaterialColor{}
	for _, it := range items {
		key := strconv.FormatUint(it.SupplierMaterialId, 10)
		grouped[key] = append(grouped[key], it)
	}
	c.JSON(http.StatusOK, grouped)
}

// PostSupplierMaterialColor doesn't upsert: a duplicate (material, code) is
// always a mistake, and idx_supplier_material_colors_dedupe rejects it.
func PostSupplierMaterialColor(c *gin.Context) {
	var newItem dto.SupplierMaterialColor
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	if newItem.SupplierMaterialId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_material_id is required"})
		return
	}
	if err := db.Where("id = ?", newItem.SupplierMaterialId).First(&dto.SupplierMaterial{}).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No material with that id"})
		return
	}

	if result := db.Create(&newItem); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed — this material may already have a colour with this code"})
		return
	}
	c.JSON(http.StatusCreated, newItem)
}

func UpdateSupplierMaterialColor(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.SupplierMaterialColor
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Colour not found"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	var body dto.SupplierMaterialColor
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Map-based (not struct-based) Updates, and raw to tell "key present
	// and null" from "key absent" — same reasoning as UpdateClientItem:
	// category is nullable, and only an explicit `"category": null` should
	// clear it.
	updates := map[string]interface{}{}
	if _, ok := raw["code"]; ok {
		updates["code"] = body.Code
	}
	if _, ok := raw["category"]; ok {
		updates["category"] = body.Category
	}
	if _, ok := raw["hex"]; ok {
		updates["hex"] = body.Hex
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			dbError(c, err)
			return
		}
	}

	var updated dto.SupplierMaterialColor
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteSupplierMaterialColor(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.SupplierMaterialColor{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Colour not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
