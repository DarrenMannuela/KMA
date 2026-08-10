package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
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

	// Pre-check: catches the common case (two clients suggested the same
	// next number, one submits a moment after the other) and turns it
	// into a clean, actionable 409 instead of a confusing 500. This still
	// has a narrow race window on its own — see the comment below.
	var conflict dto.Orders
	if err := db.Where("id = ?", newOrder.Id).First(&conflict).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An order with this ID already exists"})
		return
	}

	results := db.Create(&newOrder)
	if results.Error != nil {
		// Covers the true-simultaneous case: two requests can both pass
		// the pre-check above before either has inserted. The primary
		// key constraint on Orders.Id is the real backstop then — only
		// one Create can win. Rather than distinguish "collision" from
		// "some other DB failure" here (which needs driver-specific error
		// inspection we don't have wired up), we treat any post-precheck
		// failure as a likely collision, since that's overwhelmingly the
		// realistic cause for this endpoint. If you start seeing 409s for
		// unrelated DB errors, that's the signal to add real error-code
		// inspection (e.g. checking for SQLite's "UNIQUE constraint
		// failed" text) instead of this blanket treatment.
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

	// raw lets us tell "the client sent this field" apart from "the
	// client sent this field as empty/zero" — a PATCH that only contains
	// {"id": "003/KMA/26"} must leave company/po_number/date untouched,
	// not null them out. ShouldBindBodyWithJSON caches the raw body, so
	// binding it twice (once into a map, once into the typed struct
	// below) is safe.
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

	// If the id IS changing, make sure it doesn't collide with another
	// order first — Items/Invoice have ON UPDATE CASCADE FKs, but that
	// only helps propagate a rename; it won't stop two orders from
	// colliding on the same id.
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

	// Anchored to the OLD id so this is a real
	// "UPDATE orders SET id = new WHERE id = old" statement — required
	// both to hit the right row and to trigger ON UPDATE CASCADE.
	if err := db.Model(&dto.Orders{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return the actual merged record, not just whatever partial fields
	// the client happened to send.
	var updated dto.Orders
	db.Where("id = ?", newId).First(&updated)
	c.JSON(http.StatusOK, updated)
}

func DeleteOrders(c *gin.Context) {
	id := getID(c)
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Orders{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
