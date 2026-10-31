package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetSupplierMaterials(c *gin.Context) {
	var items []dto.SupplierMaterial
	db := Connect()

	if result := db.Find(&items); result.Error != nil {
		dbError(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, items)
}

// GetSupplierMaterialsBySupplier loads one supplier's own catalogue — same
// shape as GetClientItemsByClient.
func GetSupplierMaterialsBySupplier(c *gin.Context) {
	supplierId := c.Query("supplier_id")
	var items []dto.SupplierMaterial
	db := Connect()

	if result := db.Where("supplier_id = ?", supplierId).Find(&items); result.Error != nil {
		dbError(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetSupplierMaterialByID(c *gin.Context) {
	id := c.Param("id")
	var item dto.SupplierMaterial
	db := Connect()

	if err := db.Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier material not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// PostSupplierMaterial doesn't upsert: a duplicate (supplier_id, name) is
// always a mistake, and idx_supplier_materials_dedupe rejects it.
func PostSupplierMaterial(c *gin.Context) {
	var newItem dto.SupplierMaterial
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	if newItem.SupplierId == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "supplier_id is required"})
		return
	}
	if err := db.Where("id = ?", newItem.SupplierId).First(&dto.Supplier{}).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No supplier with that id"})
		return
	}

	if result := db.Create(&newItem); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed — this supplier may already have a material with this name"})
		return
	}
	c.JSON(http.StatusCreated, newItem)
}

func UpdateSupplierMaterial(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.SupplierMaterial
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier material not found"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	var body dto.SupplierMaterial
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// supplier_id is deliberately not updatable here — same reasoning as
	// ClientItem.client_id: a material's colours have no supplier_id of
	// their own, so relocating it isn't a supported operation.
	updates := map[string]interface{}{}
	for _, f := range []string{"name", "brand", "composition", "used_for", "width_cm", "roll_yards", "price"} {
		if _, ok := raw[f]; !ok {
			continue
		}
		switch f {
		case "name":
			updates["name"] = body.Name
		case "brand":
			updates["brand"] = body.Brand
		case "composition":
			updates["composition"] = body.Composition
		case "used_for":
			updates["used_for"] = body.UsedFor
		case "width_cm":
			updates["width_cm"] = body.WidthCm
		case "roll_yards":
			updates["roll_yards"] = body.RollYards
		case "price":
			updates["price"] = body.Price
		}
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			dbError(c, err)
			return
		}
	}

	var updated dto.SupplierMaterial
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteSupplierMaterial also removes its colours (FK cascade).
func DeleteSupplierMaterial(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.SupplierMaterial{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier material not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
