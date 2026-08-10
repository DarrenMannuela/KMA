package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetDelivery(c *gin.Context) {
	var deliveries []dto.Delivery
	db := Connect()

	results := db.Find(&deliveries)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}
	c.JSON(200, deliveries)
}

func GetDeliveryByID(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	var delivery dto.Delivery
	db := Connect()

	result := db.Where("id = ?", id).First(&delivery)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}
	c.JSON(http.StatusOK, delivery)
}

func PostDelivery(c *gin.Context) {
	var newDeliveries dto.Delivery
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newDeliveries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Same pattern as PostOrders/PostInvoice/PostFinanceHeader — Delivery
	// IDs are also client-suggested strings, so they're exposed to the
	// identical "two clients suggested the same next number" race. A
	// clean precheck turns that into an actionable 409 instead of a
	// generic "Database insert failed" 500.
	var conflict dto.Delivery
	if err := db.Where("id = ?", newDeliveries.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A delivery with this ID already exists"})
		return
	}

	results := db.Create(&newDeliveries)
	if results.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A delivery with this ID already exists"})
		return
	}
	c.JSON(201, newDeliveries)

}

func UpdateDelivery(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	var existing dto.Delivery
	if result := db.Where("id = ?", id).First(&existing); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}

	// BUG FIX: the previous version bound the request body straight onto
	// `existing` (which already has its PK set to the OLD id) and then
	// called Save(). If the client's JSON included a changed "id",
	// binding overwrote existing.Id with the NEW id, so Save() built
	// "UPDATE deliveries SET ... WHERE id = <new id>" — matching ZERO
	// rows. GORM doesn't treat 0-rows-affected as an error, so this
	// silently returned 200 OK while leaving the actual row (still under
	// the old id) completely untouched.
	//
	// Fixed the same way UpdateOrders/UpdateInvoice handle it: figure out
	// the intended new id, precheck it for collisions, then update with
	// the WHERE clause explicitly anchored to the OLD id.
	//
	// NOTE: field names below (ClientId, ClientContactId, PoNumber,
	// PhoneNumber, ContactPerson, OrderId) are inferred from the
	// frontend's Delivery interface — please confirm they match
	// dto.Delivery before deploying, since that struct wasn't provided.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Delivery
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	newId := existing.Id
	if _, ok := raw["id"]; ok && body.Id != "" {
		newId = body.Id
	}
	if newId != existing.Id {
		var conflict dto.Delivery
		if err := db.Where("id = ?", newId).First(&conflict).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "A delivery with this ID already exists"})
			return
		}
	}

	updates := map[string]interface{}{"id": newId}
	if _, ok := raw["type"]; ok {
		updates["type"] = body.Type
	}
	if _, ok := raw["client_id"]; ok {
		updates["client_id"] = body.ClientId
	}
	if _, ok := raw["client_contact_id"]; ok {
		updates["client_contact_id"] = body.ClientContactId
	}
	if _, ok := raw["company"]; ok {
		updates["company"] = body.Company
	}
	if _, ok := raw["address"]; ok {
		updates["address"] = body.Address
	}
	if _, ok := raw["po_number"]; ok {
		updates["po_number"] = body.PoNumber
	}
	if _, ok := raw["phone_number"]; ok {
		updates["phone_number"] = body.PhoneNumber
	}
	if _, ok := raw["contact_person"]; ok {
		updates["contact_person"] = body.ContactPerson
	}
	if _, ok := raw["date"]; ok {
		updates["date"] = body.Date
	}
	if _, ok := raw["order_id"]; ok {
		updates["order_id"] = body.OrderId
	}

	// Anchored to the OLD id — this is the fix. Same reasoning as
	// UpdateOrders: required both to hit the right row and to trigger any
	// ON UPDATE CASCADE if delivery_item has an FK to this id.
	if err := db.Model(&dto.Delivery{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated dto.Delivery
	db.Where("id = ?", newId).First(&updated)
	c.JSON(http.StatusOK, updated)
}

func DeleteDelivery(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Delivery{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
