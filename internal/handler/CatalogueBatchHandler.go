package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CatalogueBatch is a set of changes to one client's catalogue, saved in one
// transaction: items added with their prices (adding an item with all its
// sizes, copying from another client, an import), prices set or removed (an
// edit, a price rise across many items) and items removed (undo).
type CatalogueBatch struct {
	ClientId uint64 `json:"client_id"`
	// UpsertItems are found by name and size, or added; their prices are set.
	UpsertItems  []CatalogueItemIn `json:"upsert_items"`
	SetPrices    []PriceIn         `json:"set_prices"`
	DeletePrices []PriceKey        `json:"delete_prices"`
	DeleteItems  []uint64          `json:"delete_items"`
}

type CatalogueItemIn struct {
	ItemName string    `json:"item_name"`
	Size     string    `json:"size"`
	Notes    *string   `json:"notes"`
	Prices   []PriceIn `json:"prices"`
}

type PriceIn struct {
	ClientItemId  uint64     `json:"client_item_id"`
	Year          int        `json:"year"`
	Price         int64      `json:"price"`
	EffectiveDate *time.Time `json:"effective_date"`
}

type PriceKey struct {
	ClientItemId uint64 `json:"client_item_id"`
	Year         int    `json:"year"`
}

// PriceBefore is a price as it was before the batch (Existed false: there
// was none), which is what undo puts back.
type PriceBefore struct {
	ClientItemId  uint64     `json:"client_item_id"`
	Year          int        `json:"year"`
	Existed       bool       `json:"existed"`
	Price         int64      `json:"price"`
	EffectiveDate *time.Time `json:"effective_date"`
}

type CatalogueBatchResult struct {
	ItemsCreated []dto.ClientItem      `json:"items_created"`
	ItemsDeleted []dto.ClientItem      `json:"items_deleted"`
	PricesBefore []PriceBefore         `json:"prices_before"`
	Prices       []dto.ClientItemPrice `json:"prices"`
}

func PostCatalogueBatch(c *gin.Context) {
	var b CatalogueBatch
	if err := c.ShouldBindBodyWithJSON(&b); err != nil {
		bindError(c, err)
		return
	}
	size := len(b.UpsertItems) + len(b.SetPrices) + len(b.DeletePrices) + len(b.DeleteItems)
	for _, it := range b.UpsertItems {
		size += len(it.Prices)
	}
	if size == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to save"})
		return
	}
	if size > maxBatch {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many changes at once (%d, at most %d)", size, maxBatch)})
		return
	}

	res := CatalogueBatchResult{}
	err := Connect().Transaction(func(tx *gorm.DB) error { return applyCatalogue(tx, &b, &res) })
	var be batchError
	switch {
	case errors.As(err, &be):
		c.JSON(be.status, gin.H{"error": be.msg})
	case err != nil:
		dbError(c, err)
	default:
		// The photos of removed items go once the removal is saved.
		for _, it := range res.ItemsDeleted {
			if it.PhotoPath != nil {
				removePhotoFile(*it.PhotoPath)
			}
		}
		c.JSON(http.StatusOK, res)
	}
}

func applyCatalogue(tx *gorm.DB, b *CatalogueBatch, res *CatalogueBatchResult) error {
	if b.ClientId == 0 {
		return badBatch("client_id is required")
	}
	var n int64
	if err := tx.Model(&dto.Client{}).Where("id = ?", b.ClientId).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return batchError{http.StatusNotFound, "Client not found"}
	}
	seen := map[[2]int]bool{} // (item, year) already recorded in PricesBefore

	setPrice := func(p PriceIn) error {
		if p.Year < 1900 || p.Year > 3000 {
			return badBatch("Year %d isn't a year", p.Year)
		}
		if p.Price < 0 {
			return badBatch("A price can't be negative")
		}
		var item dto.ClientItem
		if err := tx.Where("id = ? AND client_id = ?", p.ClientItemId, b.ClientId).First(&item).Error; err != nil {
			return badBatch("Catalogue item %d isn't this client's", p.ClientItemId)
		}
		key := [2]int{int(p.ClientItemId), p.Year}
		if !seen[key] {
			seen[key] = true
			var old dto.ClientItemPrice
			before := PriceBefore{ClientItemId: p.ClientItemId, Year: p.Year}
			if tx.Where("client_item_id = ? AND year = ?", p.ClientItemId, p.Year).First(&old).Error == nil {
				before.Existed, before.Price, before.EffectiveDate = true, old.Price, old.EffectiveDate
			}
			res.PricesBefore = append(res.PricesBefore, before)
		}
		row := dto.ClientItemPrice{ClientItemId: p.ClientItemId, Year: p.Year, Price: p.Price, EffectiveDate: p.EffectiveDate}
		if err := tx.Omit("ClientItem").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "client_item_id"}, {Name: "year"}},
			DoUpdates: clause.AssignmentColumns([]string{"price", "effective_date"}),
		}).Create(&row).Error; err != nil {
			return err
		}
		var saved dto.ClientItemPrice
		tx.Where("client_item_id = ? AND year = ?", p.ClientItemId, p.Year).First(&saved)
		res.Prices = append(res.Prices, saved)
		return nil
	}

	for _, in := range b.UpsertItems {
		name := strings.ToUpper(strings.TrimSpace(in.ItemName))
		size := strings.ToUpper(strings.TrimSpace(in.Size))
		if name == "" {
			return badBatch("Every item needs a name")
		}
		var item dto.ClientItem
		// Older items may have no size at all (NULL) where newer ones have "".
		err := tx.Where("client_id = ? AND item_name = ? AND (size = ? OR (size IS NULL AND ? = ''))", b.ClientId, name, size, size).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			item = dto.ClientItem{ClientId: b.ClientId, ItemName: name, Size: &size, Notes: in.Notes}
			if err := tx.Omit("Client").Create(&item).Error; err != nil {
				return err
			}
			res.ItemsCreated = append(res.ItemsCreated, item)
		} else if err != nil {
			return err
		}
		for _, p := range in.Prices {
			p.ClientItemId = item.Id
			if err := setPrice(p); err != nil {
				return err
			}
		}
	}
	for _, p := range b.SetPrices {
		if err := setPrice(p); err != nil {
			return err
		}
	}
	for _, k := range b.DeletePrices {
		var old dto.ClientItemPrice
		if err := tx.Joins("JOIN client_items ON client_items.id = client_item_prices.client_item_id").
			Where("client_item_prices.client_item_id = ? AND client_item_prices.year = ? AND client_items.client_id = ?", k.ClientItemId, k.Year, b.ClientId).
			First(&old).Error; err != nil {
			continue // already gone
		}
		key := [2]int{int(k.ClientItemId), k.Year}
		if !seen[key] {
			seen[key] = true
			res.PricesBefore = append(res.PricesBefore, PriceBefore{ClientItemId: k.ClientItemId, Year: k.Year, Existed: true, Price: old.Price, EffectiveDate: old.EffectiveDate})
		}
		if err := tx.Where("id = ?", old.Id).Delete(&dto.ClientItemPrice{}).Error; err != nil {
			return err
		}
	}
	if len(b.DeleteItems) > 0 {
		var gone []dto.ClientItem
		if err := tx.Where("id IN ? AND client_id = ?", b.DeleteItems, b.ClientId).Find(&gone).Error; err != nil {
			return err
		}
		for _, it := range gone {
			var prices []dto.ClientItemPrice
			tx.Where("client_item_id = ?", it.Id).Find(&prices)
			for _, p := range prices {
				key := [2]int{int(p.ClientItemId), p.Year}
				if !seen[key] {
					seen[key] = true
					res.PricesBefore = append(res.PricesBefore, PriceBefore{ClientItemId: p.ClientItemId, Year: p.Year, Existed: true, Price: p.Price, EffectiveDate: p.EffectiveDate})
				}
			}
			if err := tx.Where("client_item_id = ?", it.Id).Delete(&dto.ClientItemPrice{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id = ?", it.Id).Delete(&dto.ClientItem{}).Error; err != nil {
				return err
			}
		}
		res.ItemsDeleted = gone
	}
	return nil
}
