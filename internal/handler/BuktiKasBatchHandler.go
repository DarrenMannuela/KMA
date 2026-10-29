package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BuktiKasBatch is a set of Bukti Kas changes applied in one transaction —
// same shape as FinanceBatch (see its own comment), just with a single
// item kind instead of a production/operation split, since a Bukti Kas
// line is always the same shape (an amount received, optionally for an
// order).
type BuktiKasBatch struct {
	NewHeaders   []dto.BuktiKasHeader `json:"new_headers"`
	Headers      []dto.BuktiKasHeader `json:"headers"`
	HeaderUpdate []dto.BuktiKasHeader `json:"header_update"`
	ItemCreate   []dto.BuktiKasItem   `json:"item_create"`
	ItemUpdate   []ItemPatch          `json:"item_update"`
	ItemDelete   []uint               `json:"item_delete"`
}

type BuktiKasBatchResult struct {
	HeadersCreated []dto.BuktiKasHeader `json:"headers_created"`
	HeadersDeleted []dto.BuktiKasHeader `json:"headers_deleted"`
	HeadersBefore  []dto.BuktiKasHeader `json:"headers_before"`
	Items          []dto.BuktiKasItem  `json:"items"`
	ItemsBefore    []dto.BuktiKasItem  `json:"items_before"`
	ItemsDeleted   []dto.BuktiKasItem  `json:"items_deleted"`
}

const maxBuktiKasBatch = 2000

var buktiKasItemFields = map[string]func() interface{}{
	"header_id":   func() interface{} { return new(string) },
	"description": func() interface{} { return new(string) },
	"price":       func() interface{} { return new(int64) },
	"order_id":    func() interface{} { return new(*string) },
}

func PostBuktiKasBatch(c *gin.Context) {
	var b BuktiKasBatch
	if err := c.ShouldBindBodyWithJSON(&b); err != nil {
		bindError(c, err)
		return
	}
	size := len(b.NewHeaders) + len(b.Headers) + len(b.HeaderUpdate) + len(b.ItemCreate) + len(b.ItemUpdate) + len(b.ItemDelete)
	if size == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to save"})
		return
	}
	if size > maxBuktiKasBatch {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many changes at once (%d, at most %d)", size, maxBuktiKasBatch)})
		return
	}

	res := BuktiKasBatchResult{}
	err := Connect().Transaction(func(tx *gorm.DB) error { return applyBuktiKasBatch(tx, &b, &res) })
	var be batchError
	switch {
	case errors.As(err, &be):
		c.JSON(be.status, gin.H{"error": be.msg})
	case err != nil:
		dbError(c, err)
	default:
		c.JSON(http.StatusOK, res)
	}
}

func applyBuktiKasBatch(tx *gorm.DB, b *BuktiKasBatch, res *BuktiKasBatchResult) error {
	touched := map[string]bool{}

	for _, h := range b.NewHeaders {
		if err := createBuktiKasHeader(tx, h, true, res); err != nil {
			return err
		}
	}
	for _, h := range b.Headers {
		if err := createBuktiKasHeader(tx, h, false, res); err != nil {
			return err
		}
	}

	for _, h := range b.HeaderUpdate {
		var before dto.BuktiKasHeader
		if err := tx.Where("id = ?", h.Id).First(&before).Error; err != nil {
			return batchError{http.StatusNotFound, fmt.Sprintf("Bukti Kas %s not found", h.Id)}
		}
		if h.Date == "" {
			return badBatch("Bukti Kas %s needs a date", h.Id)
		}
		if err := tx.Model(&dto.BuktiKasHeader{}).Where("id = ?", h.Id).
			Updates(map[string]interface{}{"date": h.Date, "description": h.Description}).Error; err != nil {
			return err
		}
		res.HeadersBefore = append(res.HeadersBefore, before)
	}

	for _, item := range b.ItemCreate {
		if err := needBuktiKasHeader(tx, item.HeaderId); err != nil {
			return err
		}
		item.Header = dto.BuktiKasHeader{}
		if err := tx.Omit("Header").Create(&item).Error; err != nil {
			return err
		}
		res.Items = append(res.Items, item)
	}

	for _, p := range b.ItemUpdate {
		var before dto.BuktiKasItem
		if err := tx.Where("id = ?", p.Id).First(&before).Error; err != nil {
			return batchError{http.StatusNotFound, fmt.Sprintf("Bukti Kas line %d not found", p.Id)}
		}
		if err := patchItem(tx, &dto.BuktiKasItem{}, p, buktiKasItemFields, needBuktiKasHeader); err != nil {
			return err
		}
		res.ItemsBefore = append(res.ItemsBefore, before)
	}

	if len(b.ItemDelete) > 0 {
		var gone []dto.BuktiKasItem
		if err := tx.Where("id IN ?", b.ItemDelete).Find(&gone).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", b.ItemDelete).Delete(&dto.BuktiKasItem{}).Error; err != nil {
			return err
		}
		for _, item := range gone {
			touched[item.HeaderId] = true
		}
		res.ItemsDeleted = gone
	}

	// A Bukti Kas left with no lines is removed with its last line, same as
	// FinanceBatch — but, as there, only when this request's own explicit
	// delete emptied it (touched is only set by ItemDelete above), never
	// merely because a reparent moved its last line elsewhere.
	for id := range touched {
		var n int64
		tx.Model(&dto.BuktiKasItem{}).Where("header_id = ?", id).Count(&n)
		if n > 0 {
			continue
		}
		var h dto.BuktiKasHeader
		if tx.Where("id = ?", id).First(&h).Error != nil {
			continue
		}
		if err := tx.Where("id = ?", id).Delete(&dto.BuktiKasHeader{}).Error; err != nil {
			return err
		}
		res.HeadersDeleted = append(res.HeadersDeleted, h)
	}
	return nil
}

func createBuktiKasHeader(tx *gorm.DB, h dto.BuktiKasHeader, mustBeNew bool, res *BuktiKasBatchResult) error {
	h.Id = strings.TrimSpace(h.Id)
	if h.Id == "" {
		return badBatch("A Bukti Kas needs an ID")
	}
	var existing dto.BuktiKasHeader
	if tx.Where("id = ?", h.Id).First(&existing).Error == nil {
		if mustBeNew {
			return batchError{http.StatusConflict, fmt.Sprintf("Bukti Kas %s already exists", h.Id)}
		}
		return nil
	}
	if h.Date == "" {
		return badBatch("Bukti Kas %s needs a date", h.Id)
	}
	if err := tx.Create(&h).Error; err != nil {
		return err
	}
	res.HeadersCreated = append(res.HeadersCreated, h)
	return nil
}

func needBuktiKasHeader(tx *gorm.DB, id string) error {
	if id == "" {
		return badBatch("Every line needs a Bukti Kas ID")
	}
	var n int64
	if err := tx.Model(&dto.BuktiKasHeader{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return badBatch("Bukti Kas %s doesn't exist", id)
	}
	return nil
}
