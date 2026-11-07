package dto

import "time"

// A deleted order, invoice, delivery, client or supplier, kept with
// everything its delete took along (an order's items, invoices and
// deliveries; a client's contacts and catalogue…) so the frontend's Undo can
// put it all back in one step. Rows older than trashKeep are removed.
type DeletedRecord struct {
	Id        uint64    `gorm:"primaryKey" json:"id"`
	Kind      string    `json:"kind" gorm:"not null"`
	RecordId  string    `json:"record_id" gorm:"not null"`
	Payload   string    `json:"-" gorm:"not null"`
	TrashedAt time.Time `json:"trashed_at" gorm:"index"`
}
