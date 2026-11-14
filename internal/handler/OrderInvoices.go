package handler

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"gorm.io/gorm"
)

// Rules that keep an order and its invoices agreeing, whichever screen made
// the change: its items (invoice totals follow them), its cancellation (a
// cancelled order takes no new items, deliveries or invoices), and Bukti Kas
// lines that pay one of its invoices.

// orderCancelled says whether the order exists and is cancelled. An empty
// id, or an order that isn't there, is not cancelled.
func orderCancelled(tx *gorm.DB, orderId string) (bool, error) {
	if orderId == "" {
		return false, nil
	}
	var o dto.Orders
	err := tx.Select("id", "cancelled_at").Where("id = ?", orderId).First(&o).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return o.CancelledAt != nil, nil
}

// cancelledOrderError is answered as 409 with its message shown as is.
type cancelledOrderError struct{ orderId string }

func (e cancelledOrderError) Error() string {
	return fmt.Sprintf("Order %s is cancelled. Reactivate it before changing it.", e.orderId)
}

// refuseIfCancelled returns a cancelledOrderError for a cancelled order.
func refuseIfCancelled(tx *gorm.DB, orderIds ...string) error {
	for _, id := range orderIds {
		cancelled, err := orderCancelled(tx, id)
		if err != nil {
			return err
		}
		if cancelled {
			return cancelledOrderError{id}
		}
	}
	return nil
}

// syncOrderInvoices brings an order's invoices in line with its items: the
// total is the items' sum; an unpaid DP keeps its percentage, a paid DP its
// amount (never more than the total); a Pelunasan takes the DP's amount;
// the rest and what's owed follow. The same sums the invoice page uses.
func syncOrderInvoices(tx *gorm.DB, orderId string) error {
	if orderId == "" {
		return nil
	}
	var total int64
	if err := tx.Model(&dto.Items{}).Where("order_id = ?", orderId).Select("COALESCE(SUM(sub_total), 0)").Scan(&total).Error; err != nil {
		return err
	}
	var own []dto.Invoice
	if err := tx.Where("order_id = ?", orderId).Find(&own).Error; err != nil {
		return err
	}
	var dpInv *dto.Invoice
	for i := range own {
		if own[i].Type == "dp" {
			dpInv = &own[i]
			break
		}
	}
	var dpAmount int64
	if dpInv != nil && dpInv.DownPayment != nil && *dpInv.DownPayment != 0 {
		dp := *dpInv.DownPayment
		if dpInv.Status == "paid" || dpInv.Total == 0 {
			dpAmount = min(dp, total)
		} else {
			dpAmount = int64(math.Round(float64(total) * float64(dp) / float64(dpInv.Total)))
		}
	}
	for _, inv := range own {
		dp := int64(0)
		if inv.DownPayment != nil {
			dp = *inv.DownPayment
		}
		if inv.Type == "dp" || dpInv != nil {
			dp = dpAmount
		}
		discount := int64(0)
		if inv.Discount != nil {
			discount = *inv.Discount
		}
		remaining := total - dp
		ar := max(0, remaining-discount)
		oldDp := int64(0)
		if inv.DownPayment != nil {
			oldDp = *inv.DownPayment
		}
		if inv.Total == total && oldDp == dp && inv.Remaining == remaining && inv.ArReceivable == ar {
			continue
		}
		if err := tx.Model(&dto.Invoice{}).Where("id = ?", inv.Id).Updates(map[string]interface{}{
			"total": total, "down_payment": dp, "remaining": remaining, "ar_receivable": ar,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ── Bukti Kas lines that pay an invoice ──
//
// A Bukti Kas line with invoice_id is that invoice's payment: the invoice is
// paid, dated the Bukti Kas's date (the earliest, if several lines share it).
// Only an unpaid invoice can be linked, so an invoice with linked lines was
// paid by them; when the last link goes (line deleted or unlinked) it is
// unpaid again.

// buktiKasDate reads a Bukti Kas date ("2026-10-17", or a full timestamp
// from older rows) as the moment to store as paid_date.
func buktiKasDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// checkInvoiceLink validates linking a line to an invoice: it exists, isn't
// cancelled, belongs to the line's order (when it has one), and is unpaid
// unless other lines already pay it (a payment in parts). It returns the
// invoice's order, which the line then takes.
func checkInvoiceLink(tx *gorm.DB, invoiceId string, lineOrder *string, lineId uint) (string, error) {
	var inv dto.Invoice
	if err := tx.Where("id = ?", invoiceId).First(&inv).Error; err != nil {
		return "", batchError{http.StatusBadRequest, fmt.Sprintf("Invoice %s doesn't exist", invoiceId)}
	}
	if inv.Status == "cancelled" {
		return "", batchError{http.StatusConflict, fmt.Sprintf("Invoice %s is cancelled with its order", invoiceId)}
	}
	if lineOrder != nil && *lineOrder != "" && *lineOrder != inv.OrderId {
		return "", batchError{http.StatusBadRequest, fmt.Sprintf("Invoice %s belongs to order %s, not %s", invoiceId, inv.OrderId, *lineOrder)}
	}
	if inv.Status == "paid" {
		var others int64
		if err := tx.Model(&dto.BuktiKasItem{}).Where("invoice_id = ? AND id <> ?", invoiceId, lineId).Count(&others).Error; err != nil {
			return "", err
		}
		if others == 0 {
			return "", batchError{http.StatusConflict, fmt.Sprintf("Invoice %s is already marked paid", invoiceId)}
		}
	}
	return inv.OrderId, nil
}

// reconcileInvoicePayments sets each listed invoice paid or unpaid from the
// Bukti Kas lines that link it now.
func reconcileInvoicePayments(tx *gorm.DB, invoiceIds map[string]bool) error {
	ids := make([]string, 0, len(invoiceIds))
	for id := range invoiceIds {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		var inv dto.Invoice
		if err := tx.Where("id = ?", id).First(&inv).Error; err != nil {
			continue // deleted since: nothing to set
		}
		var dates []string
		if err := tx.Table("bukti_kas_items AS i").Joins("JOIN bukti_kas_headers AS h ON h.id = i.header_id").
			Where("i.invoice_id = ?", id).Pluck("h.date", &dates).Error; err != nil {
			return err
		}
		if len(dates) == 0 {
			if inv.Status == "paid" {
				if err := tx.Model(&dto.Invoice{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "unpaid", "paid_date": nil}).Error; err != nil {
					return err
				}
			}
			continue
		}
		var paid *time.Time
		for _, d := range dates {
			if t, ok := buktiKasDate(d); ok && (paid == nil || t.Before(*paid)) {
				tt := t
				paid = &tt
			}
		}
		if err := tx.Model(&dto.Invoice{}).Where("id = ?", id).Updates(map[string]interface{}{"status": "paid", "paid_date": paid}).Error; err != nil {
			return err
		}
	}
	return nil
}

// linkedInvoices lists the invoices paid by the given lines.
func linkedInvoices(lines []dto.BuktiKasItem, into map[string]bool) {
	for _, l := range lines {
		if l.InvoiceId != nil && *l.InvoiceId != "" {
			into[*l.InvoiceId] = true
		}
	}
}
