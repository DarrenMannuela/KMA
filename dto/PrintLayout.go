package dto

import "time"

// How one kind of printed document is laid out — the invoice, the kwitansi,
// the Surat Jalan or the Delivery Order: its font, where the logo goes and
// how big, the text sizes of each part and the spacing. One row per kind,
// shared by everyone, so every computer prints the same way. Settings is
// the frontend's JSON object, kept as text: the server only checks it's an
// object of a sensible size, and the frontend fills in and limits each value.
// No row means the document's built-in layout.
type PrintLayout struct {
	DocType   string    `gorm:"primaryKey" json:"doc_type"`
	Settings  string    `gorm:"not null" json:"-"`
	UpdatedAt time.Time `json:"updated_at"`
}
