package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DarrenMannuela/KMA/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// How long a deleted record can still be put back.
const trashKeep = 30 * 24 * time.Hour

// The frontend reads this header off a delete's 204 to offer Undo.
const restoreHeader = "X-Restore-Id"

// What each delete removes, captured just before it does.

type orderSnapshot struct {
	Order           dto.Orders         `json:"order"`
	Items           []dto.Items        `json:"items"`
	Invoices        []dto.Invoice      `json:"invoices"`
	InvoiceSheets   []savedSheet       `json:"invoice_sheets,omitempty"` // their print-page formatting
	Deliveries      []dto.Delivery     `json:"deliveries"`
	DeliveryItems   []dto.DeliveryItem `json:"delivery_items"`
	ProductionLinks []uint             `json:"production_links"` // Kas Bon lines the delete unlinked
	OperationLinks  []uint             `json:"operation_links"`
}

type invoiceSnapshot struct {
	Invoice dto.Invoice `json:"invoice"`
	Sheet   *savedSheet `json:"sheet,omitempty"` // its print-page formatting, if any
}

// savedSheet is an invoice's formatting as a snapshot keeps it (dto.InvoiceSheet
// leaves its settings out of JSON, so it can't be stored as is).
type savedSheet struct {
	InvoiceId string    `json:"invoice_id"`
	Settings  string    `json:"settings"`
	UpdatedAt time.Time `json:"updated_at"`
}

func sheetsOf(tx *gorm.DB, ids []string) ([]savedSheet, error) {
	var rows []dto.InvoiceSheet
	if err := tx.Where("invoice_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]savedSheet, len(rows))
	for i, r := range rows {
		out[i] = savedSheet{InvoiceId: r.InvoiceId, Settings: r.Settings, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

type deliverySnapshot struct {
	Delivery dto.Delivery       `json:"delivery"`
	Items    []dto.DeliveryItem `json:"items"`
}

// An order or delivery that pointed at the deleted client (SET NULL on delete).
type clientLink struct {
	Id        string  `json:"id"`
	ContactId *uint64 `json:"contact_id"`
}

type clientSnapshot struct {
	Client     dto.Client            `json:"client"`
	Contacts   []dto.ClientContact   `json:"contacts"`
	Items      []dto.ClientItem      `json:"items"`
	Prices     []dto.ClientItemPrice `json:"prices"`
	Orders     []clientLink          `json:"orders"`
	Deliveries []clientLink          `json:"deliveries"`
}

type supplierSnapshot struct {
	Supplier  dto.Supplier                `json:"supplier"`
	Materials []dto.SupplierMaterial      `json:"materials"`
	Colors    []dto.SupplierMaterialColor `json:"colors"`
	Documents []dto.SupplierDocument      `json:"documents"`
	Designs   []dto.SupplierDesign        `json:"designs"`
}

// restoreConflict is a restore that would clash with what's there now; its
// message is shown to the user as is.
type restoreConflict struct{ msg string }

func (e restoreConflict) Error() string { return e.msg }

func snapshotOrder(tx *gorm.DB, id string) (orderSnapshot, error) {
	var s orderSnapshot
	if err := tx.Where("id = ?", id).First(&s.Order).Error; err != nil {
		return s, err
	}
	if err := tx.Where("order_id = ?", id).Find(&s.Items).Error; err != nil {
		return s, err
	}
	if err := tx.Where("order_id = ?", id).Find(&s.Invoices).Error; err != nil {
		return s, err
	}
	if len(s.Invoices) > 0 {
		ids := make([]string, len(s.Invoices))
		for i, inv := range s.Invoices {
			ids[i] = inv.Id
		}
		sheets, err := sheetsOf(tx, ids)
		if err != nil {
			return s, err
		}
		s.InvoiceSheets = sheets
	}
	if err := tx.Where("order_id = ?", id).Find(&s.Deliveries).Error; err != nil {
		return s, err
	}
	if len(s.Deliveries) > 0 {
		ids := make([]string, len(s.Deliveries))
		for i, d := range s.Deliveries {
			ids[i] = d.Id
		}
		if err := tx.Where("delivery_id IN ?", ids).Find(&s.DeliveryItems).Error; err != nil {
			return s, err
		}
	}
	if err := tx.Model(&dto.ProductionItem{}).Where("order_id = ?", id).Pluck("id", &s.ProductionLinks).Error; err != nil {
		return s, err
	}
	err := tx.Model(&dto.OperationItem{}).Where("order_id = ?", id).Pluck("id", &s.OperationLinks).Error
	return s, err
}

func snapshotInvoice(tx *gorm.DB, id string) (invoiceSnapshot, error) {
	var s invoiceSnapshot
	if err := tx.Where("id = ?", id).First(&s.Invoice).Error; err != nil {
		return s, err
	}
	sheets, err := sheetsOf(tx, []string{id})
	if err != nil {
		return s, err
	}
	if len(sheets) > 0 {
		s.Sheet = &sheets[0]
	}
	return s, nil
}

// putSheets brings back deleted invoices' formatting, replacing any an
// invoice of the same id was given since.
func putSheets(tx *gorm.DB, sheets []savedSheet) error {
	rows := make([]dto.InvoiceSheet, len(sheets))
	for i, sh := range sheets {
		if err := tx.Where("invoice_id = ?", sh.InvoiceId).Delete(&dto.InvoiceSheet{}).Error; err != nil {
			return err
		}
		rows[i] = dto.InvoiceSheet{InvoiceId: sh.InvoiceId, Settings: sh.Settings, UpdatedAt: sh.UpdatedAt}
	}
	return insert(tx, rows)
}

func snapshotDelivery(tx *gorm.DB, id string) (deliverySnapshot, error) {
	var s deliverySnapshot
	if err := tx.Where("id = ?", id).First(&s.Delivery).Error; err != nil {
		return s, err
	}
	err := tx.Where("delivery_id = ?", id).Find(&s.Items).Error
	return s, err
}

func snapshotClient(tx *gorm.DB, id string) (clientSnapshot, error) {
	var s clientSnapshot
	if err := tx.Where("id = ?", id).First(&s.Client).Error; err != nil {
		return s, err
	}
	if err := tx.Where("client_id = ?", id).Find(&s.Contacts).Error; err != nil {
		return s, err
	}
	if err := tx.Where("client_id = ?", id).Find(&s.Items).Error; err != nil {
		return s, err
	}
	if len(s.Items) > 0 {
		ids := make([]uint64, len(s.Items))
		for i, it := range s.Items {
			ids[i] = it.Id
		}
		if err := tx.Where("client_item_id IN ?", ids).Find(&s.Prices).Error; err != nil {
			return s, err
		}
	}
	var orders []dto.Orders
	if err := tx.Where("client_id = ?", id).Find(&orders).Error; err != nil {
		return s, err
	}
	for _, o := range orders {
		s.Orders = append(s.Orders, clientLink{Id: o.Id, ContactId: o.ClientContactId})
	}
	var deliveries []dto.Delivery
	if err := tx.Where("client_id = ?", id).Find(&deliveries).Error; err != nil {
		return s, err
	}
	for _, d := range deliveries {
		s.Deliveries = append(s.Deliveries, clientLink{Id: d.Id, ContactId: d.ClientContactId})
	}
	return s, nil
}

func snapshotSupplier(tx *gorm.DB, id string) (supplierSnapshot, error) {
	var s supplierSnapshot
	if err := tx.Where("id = ?", id).First(&s.Supplier).Error; err != nil {
		return s, err
	}
	if err := tx.Where("supplier_id = ?", id).Find(&s.Materials).Error; err != nil {
		return s, err
	}
	if len(s.Materials) > 0 {
		ids := make([]uint64, len(s.Materials))
		for i, m := range s.Materials {
			ids[i] = m.Id
		}
		if err := tx.Where("supplier_material_id IN ?", ids).Find(&s.Colors).Error; err != nil {
			return s, err
		}
	}
	if err := tx.Where("supplier_id = ?", id).Find(&s.Documents).Error; err != nil {
		return s, err
	}
	// Their photo files stay on disk with the row gone, so a restore finds them.
	err := tx.Where("supplier_id = ?", id).Find(&s.Designs).Error
	return s, err
}

// trash stores a delete's snapshot and drops anything past trashKeep.
func trash(tx *gorm.DB, kind, recordId string, snapshot any) (uint64, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return 0, err
	}
	rec := dto.DeletedRecord{Kind: kind, RecordId: recordId, Payload: string(payload), TrashedAt: time.Now()}
	if err := tx.Create(&rec).Error; err != nil {
		return 0, err
	}
	if err := tx.Where("trashed_at < ?", time.Now().Add(-trashKeep)).Delete(&dto.DeletedRecord{}).Error; err != nil {
		return 0, err
	}
	return rec.Id, nil
}

// deleteWithTrash runs one delete as: snapshot, delete, keep the snapshot,
// all in one transaction. A record that isn't there is a 404, as before; a
// delete that worked is still a 204, now carrying the restore id.
func deleteWithTrash(c *gin.Context, kind, id, notFound string, remove func(tx *gorm.DB) (any, error)) {
	var restoreId uint64
	err := Connect().Transaction(func(tx *gorm.DB) error {
		snap, err := remove(tx)
		if err != nil {
			return err
		}
		restoreId, err = trash(tx, kind, id, snap)
		return err
	})
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": notFound})
	case err != nil:
		dbError(c, err)
	default:
		c.Header(restoreHeader, strconv.FormatUint(restoreId, 10))
		c.Status(http.StatusNoContent)
	}
}

// insert creates rows exactly as they were (same ids), without touching the
// relation fields GORM would otherwise try to save alongside.
func insert[T any](tx *gorm.DB, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Omit(clause.Associations).Create(&rows).Error
}

func exists(tx *gorm.DB, model any, id any) (bool, error) {
	var n int64
	err := tx.Model(model).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

// unlinkMissing clears a restored row's client/contact link when that client
// or contact was deleted in the meantime; the row keeps its typed company
// name, like any other order or delivery whose client was removed.
func unlinkMissing(tx *gorm.DB, clientId, contactId **uint64) error {
	if *clientId != nil {
		ok, err := exists(tx, &dto.Client{}, **clientId)
		if err != nil {
			return err
		}
		if !ok {
			*clientId, *contactId = nil, nil
			return nil
		}
	}
	if *contactId != nil {
		ok, err := exists(tx, &dto.ClientContact{}, **contactId)
		if err != nil {
			return err
		}
		if !ok {
			*contactId = nil
		}
	}
	return nil
}

func restoreOrder(tx *gorm.DB, s orderSnapshot) error {
	if ok, err := exists(tx, &dto.Orders{}, s.Order.Id); err != nil || ok {
		if err != nil {
			return err
		}
		return restoreConflict{fmt.Sprintf("An order %s was added since, so the deleted one can't be put back.", s.Order.Id)}
	}
	if err := unlinkMissing(tx, &s.Order.ClientId, &s.Order.ClientContactId); err != nil {
		return err
	}
	for i := range s.Deliveries {
		if err := unlinkMissing(tx, &s.Deliveries[i].ClientId, &s.Deliveries[i].ClientContactId); err != nil {
			return err
		}
	}
	if err := insert(tx, []dto.Orders{s.Order}); err != nil {
		return err
	}
	for _, step := range []func() error{
		func() error { return insert(tx, s.Items) },
		func() error { return insert(tx, s.Invoices) },
		func() error { return putSheets(tx, s.InvoiceSheets) },
		func() error { return insert(tx, s.Deliveries) },
		func() error { return insert(tx, s.DeliveryItems) },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	// Relink only lines that are still unlinked; one linked elsewhere since keeps its new order.
	if len(s.ProductionLinks) > 0 {
		if err := tx.Model(&dto.ProductionItem{}).Where("id IN ? AND order_id IS NULL", s.ProductionLinks).Update("order_id", s.Order.Id).Error; err != nil {
			return err
		}
	}
	if len(s.OperationLinks) > 0 {
		if err := tx.Model(&dto.OperationItem{}).Where("id IN ? AND order_id IS NULL", s.OperationLinks).Update("order_id", s.Order.Id).Error; err != nil {
			return err
		}
	}
	return nil
}

func restoreInvoice(tx *gorm.DB, s invoiceSnapshot) error {
	if ok, err := exists(tx, &dto.Invoice{}, s.Invoice.Id); err != nil || ok {
		if err != nil {
			return err
		}
		return restoreConflict{fmt.Sprintf("An invoice %s was added since, so the deleted one can't be put back.", s.Invoice.Id)}
	}
	if ok, err := exists(tx, &dto.Orders{}, s.Invoice.OrderId); err != nil || !ok {
		if err != nil {
			return err
		}
		return restoreConflict{fmt.Sprintf("Order %s no longer exists; put it back first.", s.Invoice.OrderId)}
	}
	if err := insert(tx, []dto.Invoice{s.Invoice}); err != nil {
		return err
	}
	if s.Sheet != nil {
		return putSheets(tx, []savedSheet{*s.Sheet})
	}
	return nil
}

func restoreDelivery(tx *gorm.DB, s deliverySnapshot) error {
	if ok, err := exists(tx, &dto.Delivery{}, s.Delivery.Id); err != nil || ok {
		if err != nil {
			return err
		}
		return restoreConflict{fmt.Sprintf("A delivery %s was added since, so the deleted one can't be put back.", s.Delivery.Id)}
	}
	if s.Delivery.OrderId != nil {
		if ok, err := exists(tx, &dto.Orders{}, *s.Delivery.OrderId); err != nil || !ok {
			if err != nil {
				return err
			}
			return restoreConflict{fmt.Sprintf("Order %s no longer exists; put it back first.", *s.Delivery.OrderId)}
		}
	}
	if err := unlinkMissing(tx, &s.Delivery.ClientId, &s.Delivery.ClientContactId); err != nil {
		return err
	}
	if err := insert(tx, []dto.Delivery{s.Delivery}); err != nil {
		return err
	}
	return insert(tx, s.Items)
}

func restoreClient(tx *gorm.DB, s clientSnapshot) error {
	if ok, err := exists(tx, &dto.Client{}, s.Client.Id); err != nil || ok {
		if err != nil {
			return err
		}
		return restoreConflict{"That client's record number is in use again, so it can't be put back."}
	}
	if err := insert(tx, []dto.Client{s.Client}); err != nil {
		return err
	}
	if err := insert(tx, s.Contacts); err != nil {
		return err
	}
	if err := insert(tx, s.Items); err != nil {
		return err
	}
	if err := insert(tx, s.Prices); err != nil {
		return err
	}
	// Orders and deliveries the delete unlinked, unless they've been linked to another client since.
	for _, rel := range []struct {
		model any
		links []clientLink
	}{{&dto.Orders{}, s.Orders}, {&dto.Delivery{}, s.Deliveries}} {
		for _, l := range rel.links {
			err := tx.Model(rel.model).Where("id = ? AND client_id IS NULL", l.Id).
				Updates(map[string]any{"client_id": s.Client.Id, "client_contact_id": l.ContactId}).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func restoreSupplier(tx *gorm.DB, s supplierSnapshot) error {
	if ok, err := exists(tx, &dto.Supplier{}, s.Supplier.Id); err != nil || ok {
		if err != nil {
			return err
		}
		return restoreConflict{"That supplier's record number is in use again, so it can't be put back."}
	}
	if err := insert(tx, []dto.Supplier{s.Supplier}); err != nil {
		return err
	}
	if err := insert(tx, s.Materials); err != nil {
		return err
	}
	if err := insert(tx, s.Colors); err != nil {
		return err
	}
	if err := insert(tx, s.Documents); err != nil {
		return err
	}
	return insert(tx, s.Designs)
}

func restoreRecord(tx *gorm.DB, rec dto.DeletedRecord) error {
	unmarshal := func(v any) error { return json.Unmarshal([]byte(rec.Payload), v) }
	switch rec.Kind {
	case "order":
		var s orderSnapshot
		if err := unmarshal(&s); err != nil {
			return err
		}
		return restoreOrder(tx, s)
	case "invoice":
		var s invoiceSnapshot
		if err := unmarshal(&s); err != nil {
			return err
		}
		return restoreInvoice(tx, s)
	case "delivery":
		var s deliverySnapshot
		if err := unmarshal(&s); err != nil {
			return err
		}
		return restoreDelivery(tx, s)
	case "client":
		var s clientSnapshot
		if err := unmarshal(&s); err != nil {
			return err
		}
		return restoreClient(tx, s)
	case "supplier":
		var s supplierSnapshot
		if err := unmarshal(&s); err != nil {
			return err
		}
		return restoreSupplier(tx, s)
	}
	return fmt.Errorf("unknown deleted record kind %q", rec.Kind)
}

// RestoreDeleted puts a deleted record back, with everything its delete
// removed, then forgets the snapshot so it can't be restored twice.
// POST /trash/:id/restore
func RestoreDeleted(c *gin.Context) {
	id := c.Param("id")
	db := Connect()
	var rec dto.DeletedRecord
	if err := db.Where("id = ?", id).First(&rec).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "There's nothing to put back: it was already restored, or deleted too long ago."})
		return
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := restoreRecord(tx, rec); err != nil {
			return err
		}
		return tx.Delete(&rec).Error
	})
	var conflict restoreConflict
	switch {
	case errors.As(err, &conflict):
		c.JSON(http.StatusConflict, gin.H{"error": conflict.msg})
	case err != nil && strings.Contains(err.Error(), "UNIQUE"):
		c.JSON(http.StatusConflict, gin.H{"error": "Something with the same number was added since, so this can't be put back."})
	case err != nil:
		dbError(c, err)
	default:
		c.JSON(http.StatusOK, gin.H{"kind": rec.Kind, "record_id": rec.RecordId})
	}
}
