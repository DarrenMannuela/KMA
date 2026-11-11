package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// The documents that have a layout.
var printDocTypes = map[string]bool{"invoice": true, "kwitansi": true, "sj": true, "do": true}

// A layout is a handful of numbers and names; anything bigger isn't one.
const maxPrintLayoutBytes = 8 << 10

type printLayoutResponse struct {
	DocType   string          `json:"doc_type"`
	Settings  json.RawMessage `json:"settings"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func toPrintLayoutResponse(l dto.PrintLayout) printLayoutResponse {
	return printLayoutResponse{DocType: l.DocType, Settings: json.RawMessage(l.Settings), UpdatedAt: l.UpdatedAt}
}

// GetPrintLayouts lists the saved layouts; a document without one prints
// with its built-in layout.
func GetPrintLayouts(c *gin.Context) {
	var rows []dto.PrintLayout
	if err := Connect().Order("doc_type").Find(&rows).Error; err != nil {
		dbError(c, err)
		return
	}
	out := make([]printLayoutResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPrintLayoutResponse(r))
	}
	c.JSON(http.StatusOK, out)
}

// PutPrintLayout saves one document's layout for everyone: {"settings": {...}}.
func PutPrintLayout(c *gin.Context) {
	docType := c.Param("type")
	if !printDocTypes[docType] {
		c.JSON(http.StatusNotFound, gin.H{"error": "No such document"})
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
	if len(settings) > maxPrintLayoutBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings are too large"})
		return
	}
	// Stored compacted, as sent.
	var compact bytes.Buffer
	if err := json.Compact(&compact, settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings must be an object"})
		return
	}
	row := dto.PrintLayout{DocType: docType, Settings: compact.String(), UpdatedAt: time.Now()}
	if err := Connect().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "doc_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"settings", "updated_at"}),
	}).Create(&row).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, toPrintLayoutResponse(row))
}

// DeletePrintLayout puts a document back to its built-in layout.
func DeletePrintLayout(c *gin.Context) {
	docType := c.Param("type")
	if !printDocTypes[docType] {
		c.JSON(http.StatusNotFound, gin.H{"error": "No such document"})
		return
	}
	if err := Connect().Where("doc_type = ?", docType).Delete(&dto.PrintLayout{}).Error; err != nil {
		dbError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"doc_type": docType})
}
