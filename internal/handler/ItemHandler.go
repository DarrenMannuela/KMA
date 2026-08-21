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

// GetItemByID fetches a single line item by its (numeric, auto-increment)
// PK. Every other entity in this API already has this single-record GET —
// Items was the one gap: no handler existed, and main.go had no route for
// it. itemsApi.get(id)/itemHooks.useGet(id) on the frontend call this via
// the generic crud() factory, same as every other entity, so without this
// that call would 404 the moment anything actually used it.
func GetItemByID(c *gin.Context) {
	id := c.Param("id")
	var item dto.Items
	db := Connect()

	if err := db.First(&item, id).Error; err != nil {
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

	// Upsert on the idx_items_dedupe unique index (order_id, item_name,
	// size, price). If a row with those same four values already exists,
	// fold the new amount/sub_total into it instead of inserting a
	// duplicate row — this used to be a frontend-only check (only
	// covered one form, and had a race between two near-simultaneous
	// adds); doing it as a DB-level upsert makes it atomic and applies
	// to every caller (UI, scripts, bulk import, etc).
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

	// After a merge, `newItem` in memory still holds what was SENT, not
	// the merged total (the +amount/+sub_total math happened in SQL, not
	// in this struct) — re-fetch the real row so the response reflects
	// the true state.
	final := findExactItem(db, newItem.OrderId, newItem.ItemName, newItem.Size, newItem.Price)
	c.JSON(201, final)
}

// findExactItem looks up a row by the same four columns idx_items_dedupe
// covers. Size needs special handling because SQLite (and SQL generally)
// requires "IS NULL" rather than "= NULL" for a nil comparison.
func findExactItem(db *gorm.DB, orderId, itemName string, size *string, price int64) dto.Items {
	var item dto.Items
	q := db.Where("order_id = ? AND item_name = ? AND price = ?", orderId, itemName, price)
	if size != nil {
		q = q.Where("size = ?", *size)
	} else {
		q = q.Where("size IS NULL")
	}
	q.First(&item)
	return item
}

func UpdateItems(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.Items
	if err := db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
		return
	}

	// raw tells us which fields were actually included in this PATCH —
	// e.g. a request that only sends {"price": 50000} must leave
	// item_name/size/amount/order_id untouched. ShouldBindBodyWithJSON
	// caches the raw body, so binding it twice below is safe.
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

	// PostItems' upsert only merges duplicates on CREATE. A PATCH here goes
	// straight at the row by PK, so editing name/size/price into a value
	// that matches ANOTHER existing item on the same order would otherwise
	// hit the idx_items_dedupe unique index and bubble up as a raw
	// "Updates(...).Error" 500 below. Precheck it the same way PostOrders
	// prechecks id collisions, and give a clean, actionable 409 instead.
	// Only worth the extra query when a dedupe-relevant field is actually
	// part of this patch — untouched-field patches (e.g. just `amount`)
	// can't create a new collision.
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

		dupe := findExactItem(db, resultOrderId, resultItemName, resultSize, resultPrice)
		if dupe.Id != 0 && dupe.Id != existing.Id {
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
	db.First(&updated, id)
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
