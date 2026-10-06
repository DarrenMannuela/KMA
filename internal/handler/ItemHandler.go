package handler

import (
	"encoding/json"
	"net/http"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func GetItems(c *gin.Context) {
	var items []dto.Items
	db := Connect()

	results := db.Find(&items)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, items)

}

// GetItemByID fetches one line item by its numeric id.
func GetItemByID(c *gin.Context) {
	id := c.Param("id")
	var item dto.Items
	db := Connect()

	if err := db.Where("id = ?", id).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

func GetItemsByOrder(c *gin.Context) {
	id := c.Query("order_id")
	var items []dto.Items
	db := Connect()
	result := db.Where("order_id = ?", id).Find(&items)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, items)
}

func PostItems(c *gin.Context) {
	var newItem dto.Items
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newItem); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Upsert on idx_items_dedupe (order_id, item_name, size, price): the same
	// item again adds its amount and sub_total to the existing row, atomically,
	// for every caller.
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "order_id"}, {Name: "item_name"}, {Name: "size"}, {Name: "price"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"amount":    gorm.Expr("amount + ?", newItem.Amount),
			"sub_total": gorm.Expr("sub_total + ?", newItem.SubTotal),
		}),
	}).Create(&newItem)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}

	// Re-read the row: after a merge the struct still holds what was sent, not
	// the total.
	final, err := findExactItem(db, newItem.OrderId, newItem.ItemName, newItem.Size, newItem.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "item saved but could not be reloaded"})
		return
	}
	c.JSON(201, final)
}

// findExactItem looks up a row by the four idx_items_dedupe columns (a nil
// size needs IS NULL). Returns gorm.ErrRecordNotFound when nothing matches.
func findExactItem(db *gorm.DB, orderId, itemName string, size *string, price int64) (dto.Items, error) {
	var item dto.Items
	q := db.Where("order_id = ? AND item_name = ? AND price = ?", orderId, itemName, price)
	if size != nil {
		q = q.Where("size = ?", *size)
	} else {
		q = q.Where("size IS NULL")
	}
	err := q.First(&item).Error
	return item, err
}

func UpdateItems(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.Items
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// Which fields this PATCH sends; the others stay as they are. The body is
	// cached, so binding it twice is safe.
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.Items
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["order_id"]; ok {
		updates["order_id"] = body.OrderId
	}
	if _, ok := raw["item_name"]; ok {
		updates["item_name"] = body.ItemName
	}
	if _, ok := raw["size"]; ok {
		updates["size"] = body.Size
	}
	if _, ok := raw["amount"]; ok {
		updates["amount"] = body.Amount
	}
	if _, ok := raw["price"]; ok {
		updates["price"] = body.Price
	}
	if _, ok := raw["sub_total"]; ok {
		updates["sub_total"] = body.SubTotal
	}

	// An edit that makes this item equal to another one on the order would
	// break idx_items_dedupe: answer 409 instead of a 500. Only checked when a
	// dedupe field changes.
	_, orderIdChanging := raw["order_id"]
	_, itemNameChanging := raw["item_name"]
	_, sizeChanging := raw["size"]
	_, priceChanging := raw["price"]

	if orderIdChanging || itemNameChanging || sizeChanging || priceChanging {
		resultOrderId := existing.OrderId
		if orderIdChanging {
			resultOrderId = body.OrderId
		}
		resultItemName := existing.ItemName
		if itemNameChanging {
			resultItemName = body.ItemName
		}
		resultSize := existing.Size
		if sizeChanging {
			resultSize = body.Size
		}
		resultPrice := existing.Price
		if priceChanging {
			resultPrice = body.Price
		}

		// A not-found error here is the expected/normal case (no
		// collision) — only a genuine match (err == nil) is worth acting
		// on, so this intentionally doesn't treat findErr as fatal.
		dupe, findErr := findExactItem(db, resultOrderId, resultItemName, resultSize, resultPrice)
		if findErr == nil && dupe.Id != existing.Id {
			c.JSON(http.StatusConflict, gin.H{
				"error": "An item with this name, size, and price already exists on this order — adjust the quantity on that row instead of creating a duplicate",
			})
			return
		}
	}

	if len(updates) > 0 {
		// db.Model(&existing) anchors the WHERE clause to existing's
		// primary key (Id), which we never mutated — so this always
		// targets the right row regardless of what's in `updates`.
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.Items
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteItems(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.Items{})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
