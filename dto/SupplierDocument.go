package dto

import "time"

// A supplier's own catalogue or price-list PDF. A supplier can have several
// (a catalogue and a separate price list, or one per year/edition), so this
// is its own table rather than a single column on Supplier — unlike a
// client-item photo, uploading a new one adds a row instead of replacing it.
type SupplierDocument struct {
	Id         uint64    `gorm:"primaryKey" json:"id"`
	SupplierId uint64    `json:"supplier_id" gorm:"not null;index"`
	// The name it was uploaded as, for display and as the suggested download
	// filename — FilePath is the actual on-disk name (the row's own id), so
	// renaming never collides and never needs a rename on disk.
	FileName   string    `json:"file_name"`
	FilePath   string    `json:"file_path"`
	UploadedAt time.Time `json:"uploaded_at"`
	Supplier   Supplier  `json:"-" gorm:"foreignKey:SupplierId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
