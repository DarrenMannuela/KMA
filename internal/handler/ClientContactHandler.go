package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetClientContacts(c *gin.Context) {
	var contacts []dto.ClientContact
	db := Connect()

	results := db.Find(&contacts)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, contacts)
}

// GetClientContactsByClient powers the Order/Delivery form's "prefill
// from POC" dropdown — same shape as GetItemsByOrder in ItemHandler.go.
func GetClientContactsByClient(c *gin.Context) {
	clientId := c.Query("client_id")
	var contacts []dto.ClientContact
	db := Connect()

	result := db.Where("client_id = ?", clientId).Find(&contacts)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, contacts)
}

func GetClientContactByID(c *gin.Context) {
	id := c.Param("id")
	var contact dto.ClientContact
	db := Connect()

	if err := db.First(&contact, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client contact not found"})
		return
	}
	c.JSON(http.StatusOK, contact)
}

func PostClientContact(c *gin.Context) {
	var newContact dto.ClientContact
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newContact); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if result := db.Create(&newContact); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}
	c.JSON(201, newContact)
}

func UpdateClientContact(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientContact
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client contact not found"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.ClientContact
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["client_id"]; ok {
		updates["client_id"] = body.ClientId
	}
	if _, ok := raw["name"]; ok {
		updates["name"] = body.Name
	}
	if _, ok := raw["role"]; ok {
		updates["role"] = body.Role
	}
	if _, ok := raw["phone_number"]; ok {
		updates["phone_number"] = body.PhoneNumber
	}
	if _, ok := raw["email"]; ok {
		updates["email"] = body.Email
	}
	if _, ok := raw["location_label"]; ok {
		updates["location_label"] = body.LocationLabel
	}
	if _, ok := raw["address"]; ok {
		updates["address"] = body.Address
	}
	if _, ok := raw["is_primary"]; ok {
		updates["is_primary"] = body.IsPrimary
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.ClientContact
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}

func DeleteClientContact(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.ClientContact{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client contact not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
