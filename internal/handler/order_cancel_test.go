package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
)

// Cancelling an order marks its unpaid invoices cancelled and leaves paid
// ones alone; a cancelled order can't be invoiced; reactivating clears the
// reason and puts the cancelled invoices back to unpaid.
func TestCancelAndReactivateOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PATCH("/order/*id", UpdateOrders)
	r.POST("/invoice", PostInvoice)

	date := time.Date(2092, 2, 1, 0, 0, 0, 0, time.UTC)
	seed(t, &dto.Orders{Id: "960/KMA/92", Date: date})
	seed(t, &dto.Invoice{Id: "960/DP/92", OrderId: "960/KMA/92", Type: "dp", Total: 1000, Tanggal: date, Status: "paid"})
	seed(t, &dto.Invoice{Id: "960/PL/92", OrderId: "960/KMA/92", Type: "pelunasan", Total: 1000, Tanggal: date, Status: "unpaid"})
	statusOf := func(id string) string {
		var inv dto.Invoice
		Connect().Where("id = ?", id).First(&inv)
		return inv.Status
	}
	order := func() dto.Orders {
		var o dto.Orders
		Connect().Where("id = ?", "960/KMA/92").First(&o)
		return o
	}

	if w := callOn(t, r, http.MethodPatch, "/order/960%2FKMA%2F92", `{"cancelled_at": "2092-02-10T00:00:00Z", "cancel_reason": "CLIENT PULLED OUT"}`); w.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", w.Code, w.Body.String())
	}
	if o := order(); o.CancelledAt == nil || o.CancelReason == nil || *o.CancelReason != "CLIENT PULLED OUT" {
		t.Fatalf("order not cancelled: %+v", o)
	}
	if statusOf("960/DP/92") != "paid" || statusOf("960/PL/92") != "cancelled" {
		t.Fatalf("after cancel: dp %s, pelunasan %s", statusOf("960/DP/92"), statusOf("960/PL/92"))
	}
	if w := callOn(t, r, http.MethodPost, "/invoice", `{"id": "961/PL/92", "order_id": "960/KMA/92", "type": "pelunasan", "tanggal": "2092-02-11T00:00:00Z", "status": "unpaid"}`); w.Code != http.StatusConflict {
		t.Fatalf("invoice for a cancelled order: %d %s", w.Code, w.Body.String())
	}
	// An ordinary edit while cancelled leaves the cancellation alone.
	if w := callOn(t, r, http.MethodPatch, "/order/960%2FKMA%2F92", `{"po_number": "PO-1"}`); w.Code != http.StatusOK || order().CancelledAt == nil || statusOf("960/PL/92") != "cancelled" {
		t.Fatalf("edit while cancelled changed it: %d", w.Code)
	}

	if w := callOn(t, r, http.MethodPatch, "/order/960%2FKMA%2F92", `{"cancelled_at": null}`); w.Code != http.StatusOK {
		t.Fatalf("reactivate: %d %s", w.Code, w.Body.String())
	}
	if o := order(); o.CancelledAt != nil || o.CancelReason != nil {
		t.Fatalf("order still cancelled: %+v", o)
	}
	if statusOf("960/DP/92") != "paid" || statusOf("960/PL/92") != "unpaid" {
		t.Fatalf("after reactivate: dp %s, pelunasan %s", statusOf("960/DP/92"), statusOf("960/PL/92"))
	}
}
