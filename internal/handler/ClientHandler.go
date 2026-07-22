package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func GetClients(c *gin.Context) {
	var clients []dto.Client
	db := Connect()

	results := db.Find(&clients)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, clients)
}

func GetClientByID(c *gin.Context) {
	id := c.Param("id")
	var client dto.Client
	db := Connect()

	if err := db.First(&client, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}
	c.JSON(http.StatusOK, client)
}

func PostClient(c *gin.Context) {
	var newClient dto.Client
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newClient); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if result := db.Create(&newClient); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}
	c.JSON(201, newClient)
}

func UpdateClient(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.Client
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}

	// raw lets us tell "the client sent this field" apart from "the
	// client sent this field as empty/zero" — same pattern as
	// UpdateOrders/UpdateItems.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Client
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["client_name"]; ok {
		updates["client_name"] = body.ClientName
	}
	if _, ok := raw["address"]; ok {
		updates["address"] = body.Address
	}
	if _, ok := raw["notes"]; ok {
		updates["notes"] = body.Notes
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.Client
	db.First(&updated, id)
	c.JSON(http.StatusOK, updated)
}

func DeleteClient(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Client{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
