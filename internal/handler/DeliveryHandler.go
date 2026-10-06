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

	// As PostOrders: delivery IDs are suggested by clients too, so a clash
	// answers 409.
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

	// Precheck a new id and update anchored to the old one (binding onto the
	// loaded row and calling Save() would update zero rows on a rename).
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
	if err := db.Where("id = ?", newId).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
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
