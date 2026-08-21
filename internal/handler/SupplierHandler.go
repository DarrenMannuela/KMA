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
		c.JSON(http.StatusInternalServerError, gin.H{"error": results.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, suppliers)
}

func PostSupplier(c *gin.Context) {
	var supplier dto.Supplier
	db := Connect()

	if err := c.ShouldBindJSON(&supplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
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

	if err := db.First(&supplier, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, supplier)
}

// UpdateSupplier is a PATCH: loads the existing row, then merges only the
// fields present in the request body onto it.
//
// BUG FIX: the previous version bound the request body straight onto
// `supplier` (already loaded with its real PK) and called Save(). Supplier.Id
// is an auto-increment PK that was never meant to be client-settable — but
// nothing stopped a generic "send the whole edited object back" caller from
// including "id" in the payload anyway. If it did, binding would overwrite
// supplier.Id with whatever the client sent, and Save() builds its WHERE
// clause off that now-mutated struct — "UPDATE suppliers SET ... WHERE id =
// <bogus id>", matching zero rows. GORM doesn't treat 0-rows-affected as an
// error, so this would silently return 200 OK while the real row went
// untouched. Same class of bug already fixed in UpdateDelivery/
// UpdateFinanceHeader/UpdateDeliveryItem/UpdateOperationItem.
//
// Fixed the same way UpdateClient/UpdateClientContact handle it: bind into a
// raw map first so we only ever touch fields the client actually sent, and
// never include "id" as a settable column at all — Updates() is anchored via
// db.Model(&existing), which keys off existing.Id (never mutated), so this
// always targets the right row regardless of what's in the body.
func UpdateSupplier(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	var existing dto.Supplier
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
		return
	}

	// raw lets us tell "the client sent this field" apart from "the client
	// sent this field as empty/zero" — same pattern as UpdateOrders/
	// UpdateClient. ShouldBindBodyWithJSON caches the raw body, so binding
	// it twice (map, then typed struct) is safe.
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
	// Deliberately no "id" key here — Supplier.Id is an auto-increment PK
	// and was never meant to be client-set, unlike Orders/Invoice/Delivery/
	// FinanceHeader's client-chosen string ids. If a rename-by-PATCH need
	// ever comes up for suppliers, it should get the same precheck-then-
	// anchored-update treatment those handlers use, not a silent Save().

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.Supplier
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}

func DeleteSupplier(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	result := db.Delete(&dto.Supplier{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
