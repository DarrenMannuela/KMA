package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

// An invoice's sheet layout: empty until saved, saved and read back, replaced
// by a second save, refused when it isn't an object, too big, or for an
// invoice that doesn't exist.
func TestInvoiceSheet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invoice-sheet", GetInvoiceSheet)
	r.PUT("/invoice-sheet", PutInvoiceSheet)

	db := Connect()
	company := "TEST"
	order := dto.Orders{Id: "TST-SHEET/KMA/26", Company: &company}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("order: %v", err)
	}
	inv := dto.Invoice{Id: "TST-SHEET-INV/KMA/26", OrderId: order.Id, Type: "dp", KepadaYth: "TEST"}
	if err := db.Create(&inv).Error; err != nil {
		t.Fatalf("invoice: %v", err)
	}
	t.Cleanup(func() {
		db.Where("invoice_id = ?", inv.Id).Delete(&dto.InvoiceSheet{})
		db.Where("id = ?", inv.Id).Delete(&dto.Invoice{})
		db.Where("id = ?", order.Id).Delete(&dto.Orders{})
	})
	q := "/invoice-sheet?invoice_id=" + url.QueryEscape(inv.Id)

	if w := callOn(t, r, http.MethodGet, q, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"settings":null`) {
		t.Fatalf("empty: %d %s", w.Code, w.Body.String())
	}
	if w := callOn(t, r, http.MethodPut, q, `{"settings": {"cols": {"ket": {"align": "left"}}}}`); w.Code != http.StatusOK {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	if w := callOn(t, r, http.MethodPut, q, `{"settings": {"rows": {"total": {"bold": true, "height": 24}}}}`); w.Code != http.StatusOK {
		t.Fatalf("second save: %d %s", w.Code, w.Body.String())
	}
	w := callOn(t, r, http.MethodGet, q, "")
	if !strings.Contains(w.Body.String(), `"height":24`) || strings.Contains(w.Body.String(), `"ket"`) {
		t.Fatalf("read back: %s", w.Body.String())
	}
	for _, bad := range []string{`{"settings": [1]}`, `{}`, `{"settings": {"x": "` + strings.Repeat("a", 70000) + `"}}`} {
		if w := callOn(t, r, http.MethodPut, q, bad); w.Code != http.StatusBadRequest {
			t.Fatalf("should refuse %.30s: %d", bad, w.Code)
		}
	}
	if w := callOn(t, r, http.MethodPut, "/invoice-sheet?invoice_id=NOPE", `{"settings": {}}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown invoice: %d", w.Code)
	}
}

// A layout with nothing set removes the row; renaming an invoice takes its
// formatting along; deleting and restoring an invoice drops and returns it.
func TestInvoiceSheetLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invoice-sheet", GetInvoiceSheet)
	r.PUT("/invoice-sheet", PutInvoiceSheet)
	r.PUT("/invoice/*id", UpdateInvoice)
	r.DELETE("/invoice/*id", DeleteInvoice)
	r.POST("/trash/:id/restore", RestoreDeleted)

	db := Connect()
	company := "TEST"
	order := dto.Orders{Id: "TST-SHEET2/KMA/26", Company: &company}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("order: %v", err)
	}
	inv := dto.Invoice{Id: "TST-SHEET2-A/KMA/26", OrderId: order.Id, Type: "dp", KepadaYth: "TEST"}
	if err := db.Create(&inv).Error; err != nil {
		t.Fatalf("invoice: %v", err)
	}
	renamed := "TST-SHEET2-B/KMA/26"
	t.Cleanup(func() {
		db.Where("invoice_id IN ?", []string{inv.Id, renamed}).Delete(&dto.InvoiceSheet{})
		db.Where("id IN ?", []string{inv.Id, renamed}).Delete(&dto.Invoice{})
		db.Where("id = ?", order.Id).Delete(&dto.Orders{})
	})
	sheetOf := func(id string) int64 { return count(t, &dto.InvoiceSheet{}, "invoice_id = ?", id) }
	q := func(id string) string { return "/invoice-sheet?invoice_id=" + url.QueryEscape(id) }

	callOn(t, r, http.MethodPut, q(inv.Id), `{"settings": {"cols": {"ket": {"bold": true}}}}`)
	if sheetOf(inv.Id) != 1 {
		t.Fatal("save didn't store a row")
	}
	w := callOn(t, r, http.MethodPut, q(inv.Id), `{"settings": {"widths": {}, "folded": [], "cols": {}, "rows": {}, "cells": {}, "texts": {}, "merges": {}}}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"settings":null`) || sheetOf(inv.Id) != 0 {
		t.Fatalf("empty layout should remove the row: %d %s", w.Code, w.Body.String())
	}

	callOn(t, r, http.MethodPut, q(inv.Id), `{"settings": {"texts": {"head|no": "NO"}}}`)
	// A leftover row under the new number must give way, not block the rename.
	db.Create(&dto.InvoiceSheet{InvoiceId: renamed, Settings: `{"old":1}`})
	if w := callOn(t, r, http.MethodPut, "/invoice/"+url.PathEscape(inv.Id), `{"id": "`+renamed+`"}`); w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}
	if sheetOf(inv.Id) != 0 || count(t, &dto.InvoiceSheet{}, "invoice_id = ? AND settings LIKE ?", renamed, "%head|no%") != 1 {
		t.Fatal("rename didn't move the formatting")
	}

	undo := deleteForRestore(t, r, "/invoice/"+url.PathEscape(renamed))
	if sheetOf(renamed) != 0 {
		t.Fatal("delete left the formatting")
	}
	restore(t, r, undo, http.StatusOK)
	if count(t, &dto.InvoiceSheet{}, "invoice_id = ? AND settings LIKE ?", renamed, "%head|no%") != 1 {
		t.Fatal("restore didn't bring the formatting back")
	}
}
