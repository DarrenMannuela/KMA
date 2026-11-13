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
