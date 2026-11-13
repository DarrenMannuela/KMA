package dto

import "time"

// How one invoice's item table is laid out and formatted on its print page,
// shared by everyone: column widths, items folded to one line, and cell /
// row / column formatting (bold, italic, alignment, fill, row height).
// Settings is the frontend's JSON object, kept as text; nothing here is
// the invoice's data, only how it's shown. No row means the defaults.
type InvoiceSheet struct {
	InvoiceId string    `gorm:"primaryKey" json:"invoice_id"`
	Settings  string    `gorm:"not null" json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}
