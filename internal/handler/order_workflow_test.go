package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

func workflowRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/item", PostItems)
	r.PATCH("/item/:id", UpdateItems)
	r.DELETE("/item/:id", DeleteItems)
	r.PATCH("/order/*id", UpdateOrders)
	r.POST("/delivery", PostDelivery)
	r.PATCH("/invoice/*id", UpdateInvoice)
	r.POST("/bukti-kas/batch", PostBuktiKasBatch)
	return r
}

func invoiceRow(t *testing.T, id string) dto.Invoice {
	t.Helper()
	var inv dto.Invoice
	if err := Connect().Where("id = ?", id).First(&inv).Error; err != nil {
		t.Fatalf("invoice %s: %v", id, err)
	}
	return inv
}

// Item changes from any screen keep the order's invoices in line: an unpaid
// DP keeps its percentage, the Pelunasan takes the DP's amount.
func TestItemChangesSyncInvoices(t *testing.T) {
	r := workflowRouter()
	date := time.Date(2093, 1, 5, 0, 0, 0, 0, time.UTC)
	dp, zero := int64(500), int64(0)
	seed(t, &dto.Orders{Id: "970/KMA/93", Date: date})
	seed(t, &dto.Items{OrderId: "970/KMA/93", ItemName: "KAOS", Amount: 10, Price: 100, SubTotal: 1000})
	seed(t, &dto.Invoice{Id: "970/DP/93", OrderId: "970/KMA/93", Type: "dp", Total: 1000, DownPayment: &dp, Remaining: 500, ArReceivable: 500, Tanggal: date, Status: "unpaid"})
	seed(t, &dto.Invoice{Id: "970/PL/93", OrderId: "970/KMA/93", Type: "pelunasan", Total: 1000, DownPayment: &dp, Discount: &zero, Remaining: 500, ArReceivable: 500, Tanggal: date, Status: "unpaid"})

	w := callOn(t, r, http.MethodPost, "/item", `{"order_id": "970/KMA/93", "item_name": "TOPI", "amount": 10, "price": 100, "sub_total": 1000}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("add item: %d %s", w.Code, w.Body.String())
	}
	d, p := invoiceRow(t, "970/DP/93"), invoiceRow(t, "970/PL/93")
	if d.Total != 2000 || *d.DownPayment != 1000 || p.Total != 2000 || *p.DownPayment != 1000 || p.ArReceivable != 1000 {
		t.Fatalf("after add: dp %d/%d, pelunasan %d/%d/%d", d.Total, *d.DownPayment, p.Total, *p.DownPayment, p.ArReceivable)
	}
	var topi dto.Items
	Connect().Where("order_id = ? AND item_name = ?", "970/KMA/93", "TOPI").First(&topi)
	if w := callOn(t, r, http.MethodDelete, "/item/"+itoa(uint(topi.Id)), ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete item: %d %s", w.Code, w.Body.String())
	}
	if d := invoiceRow(t, "970/DP/93"); d.Total != 1000 || *d.DownPayment != 500 {
		t.Fatalf("after delete: dp %d/%d", d.Total, *d.DownPayment)
	}

	// A cancelled order takes no item changes and no new delivery.
	callOn(t, r, http.MethodPatch, "/order/970%2FKMA%2F93", `{"cancelled_at": "2093-02-01T00:00:00Z"}`)
	if w := callOn(t, r, http.MethodPost, "/item", `{"order_id": "970/KMA/93", "item_name": "TOPI", "amount": 1, "price": 1, "sub_total": 1}`); w.Code != http.StatusConflict {
		t.Fatalf("item on a cancelled order: %d", w.Code)
	}
	if w := callOn(t, r, http.MethodPost, "/delivery", `{"id": "970/DO/93", "type": "do", "address": "X", "date": "2093-02-02T00:00:00Z", "order_id": "970/KMA/93"}`); w.Code != http.StatusConflict {
		t.Fatalf("delivery for a cancelled order: %d %s", w.Code, w.Body.String())
	}
}

// A Bukti Kas line linked to an invoice pays it (dated the Bukti Kas), the
// invoice can't then be marked unpaid by hand, and unlinking makes it unpaid
// again. A paid invoice can't be linked.
func TestBuktiKasPaysInvoice(t *testing.T) {
	r := workflowRouter()
	date := time.Date(2093, 3, 1, 0, 0, 0, 0, time.UTC)
	seed(t, &dto.Orders{Id: "971/KMA/93", Date: date})
	seed(t, &dto.Invoice{Id: "971/PL/93", OrderId: "971/KMA/93", Type: "pelunasan", Total: 800, Remaining: 800, ArReceivable: 800, Tanggal: date, Status: "unpaid"})
	seed(t, &dto.Invoice{Id: "972/PL/93", OrderId: "971/KMA/93", Type: "dp", Total: 800, Tanggal: date, Status: "paid"})

	w := callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"new_headers": [{"id": "90/BK/KMA/93", "date": "2093-03-10"}], "item_create": [{"header_id": "90/BK/KMA/93", "description": "PELUNASAN TUNAI", "price": 800, "invoice_id": "971/PL/93"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("link: %d %s", w.Code, w.Body.String())
	}
	inv := invoiceRow(t, "971/PL/93")
	if inv.Status != "paid" || inv.PaidDate == nil || inv.PaidDate.Format("2006-01-02") != "2093-03-10" {
		t.Fatalf("not paid by the line: %s %v", inv.Status, inv.PaidDate)
	}
	var line dto.BuktiKasItem
	Connect().Where("invoice_id = ?", "971/PL/93").First(&line)
	if line.OrderId == nil || *line.OrderId != "971/KMA/93" {
		t.Fatalf("line didn't take the invoice's order: %v", line.OrderId)
	}
	if w := callOn(t, r, http.MethodPatch, "/invoice/971%2FPL%2F93", `{"status": "unpaid"}`); w.Code != http.StatusConflict {
		t.Fatalf("manual unpaid while linked: %d", w.Code)
	}
	// A new Bukti Kas date moves the paid date.
	callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"header_update": [{"id": "90/BK/KMA/93", "date": "2093-03-12", "description": ""}]}`)
	if d := invoiceRow(t, "971/PL/93").PaidDate; d == nil || d.Format("2006-01-02") != "2093-03-12" {
		t.Fatalf("paid date didn't follow: %v", d)
	}
	// Unlinking makes it unpaid again.
	w = callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"item_update": [{"id": `+itoa(line.Id)+`, "fields": {"invoice_id": null}}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("unlink: %d %s", w.Code, w.Body.String())
	}
	if inv := invoiceRow(t, "971/PL/93"); inv.Status != "unpaid" || inv.PaidDate != nil {
		t.Fatalf("after unlink: %s %v", inv.Status, inv.PaidDate)
	}
	// An invoice already paid another way can't be linked.
	w = callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"item_update": [{"id": `+itoa(line.Id)+`, "fields": {"invoice_id": "972/PL/93"}}]}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "already marked paid") {
		t.Fatalf("link to a paid invoice: %d %s", w.Code, w.Body.String())
	}
	// Deleting a linked line unpays.
	callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"item_update": [{"id": `+itoa(line.Id)+`, "fields": {"invoice_id": "971/PL/93"}}]}`)
	if invoiceRow(t, "971/PL/93").Status != "paid" {
		t.Fatal("relink didn't pay")
	}
	callOn(t, r, http.MethodPost, "/bukti-kas/batch", `{"item_delete": [`+itoa(line.Id)+`]}`)
	if invoiceRow(t, "971/PL/93").Status != "unpaid" {
		t.Fatal("deleting the line didn't unpay")
	}
}
