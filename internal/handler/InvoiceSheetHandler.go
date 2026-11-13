package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Formatting for a long invoice (every cell of 40 lines) stays well under this.
const maxInvoiceSheetBytes = 64 << 10

type invoiceSheetResponse struct {
	InvoiceId string          `json:"invoice_id"`
	Settings  json.RawMessage `json:"settings"`
	UpdatedAt *time.Time      `json:"updated_at"`
}

// GetInvoiceSheet answers ?invoice_id=… with the saved layout, or settings
// null when the invoice still has the defaults. (A query parameter, not a
// path segment: invoice ids contain slashes.)
func GetInvoiceSheet(c *gin.Context) {
	id := strings.TrimSpace(c.Query("invoice_id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invoice_id query param is required"})
		return
	}
	var row dto.InvoiceSheet
	err := Connect().Where("invoice_id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, invoiceSheetResponse{InvoiceId: id, Settings: json.RawMessage("null")})
		return
	}
	if err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, invoiceSheetResponse{InvoiceId: id, Settings: json.RawMessage(row.Settings), UpdatedAt: &row.UpdatedAt})
}

// PutInvoiceSheet saves ?invoice_id=…'s layout for everyone: {"settings": {...}}.
func PutInvoiceSheet(c *gin.Context) {
	id := strings.TrimSpace(c.Query("invoice_id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invoice_id query param is required"})
		return
	}
	db := Connect()
	if err := db.Where("id = ?", id).First(&dto.Invoice{}).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No invoice with that id"})
		return
	}
	var body struct {
		Settings json.RawMessage `json:"settings"`
	}
	if err := c.ShouldBindBodyWithJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	settings := bytes.TrimSpace(body.Settings)
	var asObject map[string]any
	if len(settings) == 0 || settings[0] != '{' || json.Unmarshal(settings, &asObject) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings must be an object"})
		return
	}
	if len(settings) > maxInvoiceSheetBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings are too large"})
		return
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings must be an object"})
		return
	}
	row := dto.InvoiceSheet{InvoiceId: id, Settings: compact.String(), UpdatedAt: time.Now()}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "invoice_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"settings", "updated_at"}),
	}).Create(&row).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, invoiceSheetResponse{InvoiceId: id, Settings: json.RawMessage(row.Settings), UpdatedAt: &row.UpdatedAt})
}
