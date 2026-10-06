package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

func GetClientItemPrices(c *gin.Context) {
	var prices []dto.ClientItemPrice
	db := Connect()

	results := db.Order("year asc").Find(&prices)
	if results.Error != nil {
		c.JSON(500, gin.H{"error": results.Error.Error()})
		return
	}

	c.JSON(200, prices)
}

// GetClientItemPricesByItem returns one item's full year-by-year history,
// oldest first — what the catalogue page's price-history view renders.
func GetClientItemPricesByItem(c *gin.Context) {
	clientItemId := c.Query("client_item_id")
	var prices []dto.ClientItemPrice
	db := Connect()

	result := db.Where("client_item_id = ?", clientItemId).Order("year asc").Find(&prices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(200, prices)
}

// GetClientItemPricesGrouped returns { [client_item_id]: Price[] } in one call.
func GetClientItemPricesGrouped(c *gin.Context) {
	var prices []dto.ClientItemPrice
	db := Connect()

	if err := db.Order("year asc").Find(&prices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	grouped := make(map[string][]dto.ClientItemPrice)
	for _, p := range prices {
		key := strconv.FormatUint(p.ClientItemId, 10)
		grouped[key] = append(grouped[key], p)
	}
	c.JSON(200, grouped)
}

// PostClientItemPrice upserts on (client_item_id, year): a year that already
// has a price is corrected in place.
func PostClientItemPrice(c *gin.Context) {
	var newPrice dto.ClientItemPrice
	db := Connect()

	if err := c.ShouldBindBodyWithJSON(&newPrice); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "client_item_id"}, {Name: "year"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"price", "effective_date"}),
	}).Create(&newPrice)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database insert failed"})
		return
	}

	var final dto.ClientItemPrice
	if err := db.Where("client_item_id = ? AND year = ?", newPrice.ClientItemId, newPrice.Year).First(&final).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "price saved but the record could not be reloaded"})
		return
	}
	c.JSON(201, final)
}

// An edit that collides with another year's row for the item answers 409
// instead of a unique-index 500. Only checked when item or year changes.
func UpdateClientItemPrice(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	var existing dto.ClientItemPrice
	if err := db.Where("id = ?", id).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item price not found"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWithJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	var body dto.ClientItemPrice
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	updates := map[string]interface{}{}
	if _, ok := raw["client_item_id"]; ok {
		updates["client_item_id"] = body.ClientItemId
	}
	if _, ok := raw["year"]; ok {
		updates["year"] = body.Year
	}
	if _, ok := raw["price"]; ok {
		updates["price"] = body.Price
	}
	if _, ok := raw["effective_date"]; ok {
		updates["effective_date"] = body.EffectiveDate
	}

	_, itemChanging := raw["client_item_id"]
	_, yearChanging := raw["year"]

	if itemChanging || yearChanging {
		resultItemId := existing.ClientItemId
		if itemChanging {
			resultItemId = body.ClientItemId
		}
		resultYear := existing.Year
		if yearChanging {
			resultYear = body.Year
		}

		var dupe dto.ClientItemPrice
		err := db.Where("client_item_id = ? AND year = ? AND id != ?", resultItemId, resultYear, existing.Id).First(&dupe).Error
		if err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"error": "This item already has a price recorded for that year — edit that row instead of creating a duplicate",
			})
			return
		}
	}

	if len(updates) > 0 {
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var updated dto.ClientItemPrice
	if err := db.Where("id = ?", id).First(&updated).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update saved but the record could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func DeleteClientItemPrice(c *gin.Context) {
	id := c.Param("id")
	db := Connect()

	result := db.Where("id = ?", id).Delete(&dto.ClientItemPrice{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Client item price not found"})
		return
	}
	c.Status(http.StatusNoContent)
}
