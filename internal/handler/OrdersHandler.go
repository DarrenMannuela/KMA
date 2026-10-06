package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetOrders(c *gin.Context) {
	var orders []dto.Orders
	db := Connect()

	results := db.Find(&orders)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, orders)

}

func GetOrderByID(c *gin.Context) {
	id := getID(c)
	var order dto.Orders
	db := Connect()

	if err := db.Where("id = ?", id).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

func PostOrders(c *gin.Context) {
	var newOrder dto.Orders
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Precheck: two clients suggesting the same next number get a 409, not a 500.
	var conflict dto.Orders
	if err := db.Where("id = ?", newOrder.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An order with this ID already exists"})
		return
	}

	results := db.Create(&newOrder)
	if results.Error != nil {
		// Two requests can both pass the precheck; the primary key then lets only one
		// in, and any create failure here is treated as that collision.
		c.JSON(http.StatusConflict, gin.H{"error": "An order with this ID already exists"})
		return
	}
	c.JSON(201, newOrder)
}

func UpdateOrders(c *gin.Context) {
	id := getID(c)
	var existing dto.Orders
	db := Connect()

	// Find existing record first
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Which fields this PATCH sends: {"id": …} alone must leave the rest as is.
	// The body is cached, so binding it twice is safe.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Orders
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	newId := existing.Id
	if _, ok := raw["id"]; ok && body.Id != "" {
		newId = body.Id
	}

	// A new id mustn't collide with another order (ON UPDATE CASCADE only
	// carries a rename along).
	if newId != existing.Id {
		var conflict dto.Orders
		if err := db.Where("id = ?", newId).First(&conflict).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "An order with this ID already exists"})
			return
		}
	}

	// Only include a column if the client's JSON actually contained that
	// key — everything else keeps its existing value.
	updates := map[string]interface{}{"id": newId}
	if _, ok := raw["company"]; ok {
		updates["company"] = body.Company
	}
	if _, ok := raw["po_number"]; ok {
		updates["po_number"] = body.PoNumber
	}
	if _, ok := raw["date"]; ok {
		updates["date"] = body.Date
	}
	if _, ok := raw["client_id"]; ok {
		updates["client_id"] = body.ClientId
	}
	if _, ok := raw["client_contact_id"]; ok {
		updates["client_contact_id"] = body.ClientContactId
	}

	// Anchored to the old id, so it renames the row and fires ON UPDATE
	// CASCADE. Cost lines point at an order without a foreign key, so a rename
	// carries them along here, in the same transaction.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dto.Orders{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return err
		}
		if newId == existing.Id {
			return nil
		}
		return relinkCosts(tx, existing.Id, &newId)
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the actual merged record, not just whatever partial fields
	// the client happened to send.
	var updated dto.Orders
	if err := db.Where("id = ?", newId).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteOrders(c *gin.Context) {
	id := getID(c)
	db := Connect()

	var result *gorm.DB
	err := db.Transaction(func(tx *gorm.DB) error {
		result = tx.Where("id = ?", id).Delete(&dto.Orders{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return relinkCosts(tx, id, nil) // the costs stay, unlinked
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// relinkCosts moves every cost line linked to order `from` to order `to`
// (nil unlinks them).
func relinkCosts(tx *gorm.DB, from string, to *string) error {
	for _, model := range []interface{}{&dto.ProductionItem{}, &dto.OperationItem{}} {
		if err := tx.Model(model).Where("order_id = ?", from).Update("order_id", to).Error; err != nil {
			return err
		}
	}
	return nil
}
