package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FinanceBatch is a set of Kas Bon changes applied in one transaction: a whole
// new Kas Bon with its lines, rows pasted or imported from a spreadsheet, a
// bulk edit or delete, or the undo of any of these. Either all of it is
// saved or none of it.
type FinanceBatch struct {
	// NewHeaders must not exist yet (a new Kas Bon); Headers are created if
	// missing and otherwise left as they are (adding lines to a Kas Bon).
	NewHeaders       []dto.FinanceHeader  `json:"new_headers"`
	Headers          []dto.FinanceHeader  `json:"headers"`
	HeaderUpdate     []dto.FinanceHeader  `json:"header_update"` // new date and description
	ProductionCreate []dto.ProductionItem `json:"production_create"`
	OperationCreate  []dto.OperationItem  `json:"operation_create"`
	ProductionUpdate []ItemPatch          `json:"production_update"`
	OperationUpdate  []ItemPatch          `json:"operation_update"`
	ProductionDelete []uint               `json:"production_delete"`
	OperationDelete  []uint               `json:"operation_delete"`
}

// ItemPatch changes some fields of one line; fields not listed keep their value.
type ItemPatch struct {
	Id     uint                       `json:"id"`
	Fields map[string]json.RawMessage `json:"fields"`
}

// FinanceBatchResult has what was created, and every changed or deleted row
// as it was before, which is all a client needs to undo the batch.
type FinanceBatchResult struct {
	HeadersCreated    []dto.FinanceHeader  `json:"headers_created"`
	HeadersDeleted    []dto.FinanceHeader  `json:"headers_deleted"`
	HeadersBefore     []dto.FinanceHeader  `json:"headers_before"`
	Production        []dto.ProductionItem `json:"production"`
	Operation         []dto.OperationItem  `json:"operation"`
	ProductionBefore  []dto.ProductionItem `json:"production_before"`
	OperationBefore   []dto.OperationItem  `json:"operation_before"`
	ProductionDeleted []dto.ProductionItem `json:"production_deleted"`
	OperationDeleted  []dto.OperationItem  `json:"operation_deleted"`
}

// maxBatch keeps one request to a size SQLite writes in well under a second.
const maxBatch = 2000

// The fields a patch may change, with the type each must decode as.
var productionFields = map[string]func() interface{}{
	"header_id": func() interface{} { return new(string) }, "material_name": func() interface{} { return new(string) },
	"price": func() interface{} { return new(int64) }, "si_unit": func() interface{} { return new(string) },
	"amount": func() interface{} { return new(float64) }, "supplier_id": func() interface{} { return new(int) },
	"order_id": func() interface{} { return new(*string) },
}

var operationFields = map[string]func() interface{}{
	"header_id": func() interface{} { return new(string) }, "category": func() interface{} { return new(string) },
	"description": func() interface{} { return new(string) }, "price": func() interface{} { return new(int64) },
	"order_id": func() interface{} { return new(*string) },
}

type batchError struct {
	status int
	msg    string
}

func (e batchError) Error() string { return e.msg }

func badBatch(format string, a ...interface{}) error {
	return batchError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}

func PostFinanceBatch(c *gin.Context) {
	var b FinanceBatch
	if err := c.ShouldBindBodyWithJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}
	size := len(b.NewHeaders) + len(b.Headers) + len(b.HeaderUpdate) + len(b.ProductionCreate) + len(b.OperationCreate) +
		len(b.ProductionUpdate) + len(b.OperationUpdate) + len(b.ProductionDelete) + len(b.OperationDelete)
	if size == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nothing to save"})
		return
	}
	if size > maxBatch {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Too many changes at once (%d, at most %d)", size, maxBatch)})
		return
	}

	res := FinanceBatchResult{}
	err := Connect().Transaction(func(tx *gorm.DB) error { return applyBatch(tx, &b, &res) })
	var be batchError
	switch {
	case errors.As(err, &be):
		c.JSON(be.status, gin.H{"error": be.msg})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Saving failed: " + err.Error()})
	default:
		c.JSON(http.StatusOK, res)
	}
}

func applyBatch(tx *gorm.DB, b *FinanceBatch, res *FinanceBatchResult) error {
	touched := map[string]bool{} // headers that lost a line
	for _, h := range b.NewHeaders {
		if err := createHeader(tx, h, true, res); err != nil {
			return err
		}
	}
	for _, h := range b.Headers {
		if err := createHeader(tx, h, false, res); err != nil {
			return err
		}
	}

	for _, h := range b.HeaderUpdate {
		var before dto.FinanceHeader
		if err := tx.Where("id = ?", h.Id).First(&before).Error; err != nil {
			return batchError{http.StatusNotFound, fmt.Sprintf("Kas Bon %s not found", h.Id)}
		}
		if h.Date == "" {
			return badBatch("Kas Bon %s needs a date", h.Id)
		}
		if err := tx.Model(&dto.FinanceHeader{}).Where("id = ?", h.Id).
			Updates(map[string]interface{}{"date": h.Date, "description": h.Description}).Error; err != nil {
			return err
		}
		res.HeadersBefore = append(res.HeadersBefore, before)
	}

	for _, item := range b.ProductionCreate {
		if err := needHeader(tx, item.HeaderId); err != nil {
			return err
		}
		if err := needSupplier(tx, item.SupplierId); err != nil {
			return err
		}
		item.Header, item.Supplier = dto.FinanceHeader{}, dto.Supplier{}
		if err := tx.Omit("Header", "Supplier").Create(&item).Error; err != nil {
			return err
		}
		res.Production = append(res.Production, item)
	}
	for _, item := range b.OperationCreate {
		if err := needHeader(tx, item.HeaderId); err != nil {
			return err
		}
		item.Header = dto.FinanceHeader{}
		if err := tx.Omit("Header").Create(&item).Error; err != nil {
			return err
		}
		res.Operation = append(res.Operation, item)
	}

	for _, p := range b.ProductionUpdate {
		var before dto.ProductionItem
		if err := tx.Where("id = ?", p.Id).First(&before).Error; err != nil {
			return batchError{http.StatusNotFound, fmt.Sprintf("Production line %d not found", p.Id)}
		}
		if err := patchItem(tx, &dto.ProductionItem{}, p, productionFields); err != nil {
			return err
		}
		res.ProductionBefore = append(res.ProductionBefore, before)
		touched[before.HeaderId] = true
	}
	for _, p := range b.OperationUpdate {
		var before dto.OperationItem
		if err := tx.Where("id = ?", p.Id).First(&before).Error; err != nil {
			return batchError{http.StatusNotFound, fmt.Sprintf("Operation line %d not found", p.Id)}
		}
		if err := patchItem(tx, &dto.OperationItem{}, p, operationFields); err != nil {
			return err
		}
		res.OperationBefore = append(res.OperationBefore, before)
		touched[before.HeaderId] = true
	}

	if len(b.ProductionDelete) > 0 {
		var gone []dto.ProductionItem
		if err := tx.Where("id IN ?", b.ProductionDelete).Find(&gone).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", b.ProductionDelete).Delete(&dto.ProductionItem{}).Error; err != nil {
			return err
		}
		for _, item := range gone {
			touched[item.HeaderId] = true
		}
		res.ProductionDeleted = gone
	}
	if len(b.OperationDelete) > 0 {
		var gone []dto.OperationItem
		if err := tx.Where("id IN ?", b.OperationDelete).Find(&gone).Error; err != nil {
			return err
		}
		if err := tx.Where("id IN ?", b.OperationDelete).Delete(&dto.OperationItem{}).Error; err != nil {
			return err
		}
		for _, item := range gone {
			touched[item.HeaderId] = true
		}
		res.OperationDeleted = gone
	}

	// A Kas Bon left with no lines on either side is removed with its last line.
	for id := range touched {
		var n int64
		tx.Model(&dto.ProductionItem{}).Where("header_id = ?", id).Count(&n)
		if n > 0 {
			continue
		}
		tx.Model(&dto.OperationItem{}).Where("header_id = ?", id).Count(&n)
		if n > 0 {
			continue
		}
		var h dto.FinanceHeader
		if tx.Where("id = ?", id).First(&h).Error != nil {
			continue
		}
		if err := tx.Where("id = ?", id).Delete(&dto.FinanceHeader{}).Error; err != nil {
			return err
		}
		res.HeadersDeleted = append(res.HeadersDeleted, h)
	}
	return nil
}

func createHeader(tx *gorm.DB, h dto.FinanceHeader, mustBeNew bool, res *FinanceBatchResult) error {
	h.Id = strings.TrimSpace(h.Id)
	if h.Id == "" {
		return badBatch("A Kas Bon needs an ID")
	}
	var existing dto.FinanceHeader
	if tx.Where("id = ?", h.Id).First(&existing).Error == nil {
		if mustBeNew {
			return batchError{http.StatusConflict, fmt.Sprintf("Kas Bon %s already exists", h.Id)}
		}
		return nil
	}
	if h.Date == "" {
		return badBatch("Kas Bon %s needs a date", h.Id)
	}
	if err := tx.Create(&h).Error; err != nil {
		return err
	}
	res.HeadersCreated = append(res.HeadersCreated, h)
	return nil
}

func needHeader(tx *gorm.DB, id string) error {
	if id == "" {
		return badBatch("Every line needs a Kas Bon ID")
	}
	var n int64
	if err := tx.Model(&dto.FinanceHeader{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return badBatch("Kas Bon %s doesn't exist", id)
	}
	return nil
}

// patchItem applies the allowed fields of p to one row of model's table.
func patchItem(tx *gorm.DB, model interface{}, p ItemPatch, allowed map[string]func() interface{}) error {
	updates := map[string]interface{}{}
	for k, raw := range p.Fields {
		mk, ok := allowed[k]
		if !ok {
			return badBatch("Field %q can't be changed", k)
		}
		v := mk()
		if err := json.Unmarshal(raw, v); err != nil {
			return badBatch("Field %q: %v", k, err)
		}
		switch val := v.(type) {
		case *string:
			updates[k] = *val
		case *int64:
			updates[k] = *val
		case *int:
			updates[k] = *val
		case *float64:
			updates[k] = *val
		case **string:
			updates[k] = *val
		}
	}
	if len(updates) == 0 {
		return nil
	}
	if id, ok := updates["header_id"].(string); ok {
		if err := needHeader(tx, id); err != nil {
			return err
		}
	}
	if id, ok := updates["supplier_id"].(int); ok {
		if err := needSupplier(tx, id); err != nil {
			return err
		}
	}
	return tx.Model(model).Where("id = ?", p.Id).Updates(updates).Error
}

func needSupplier(tx *gorm.DB, id int) error {
	var n int64
	if err := tx.Model(&dto.Supplier{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return badBatch("Pick a supplier for every production line (supplier %d doesn't exist)", id)
	}
	return nil
}
