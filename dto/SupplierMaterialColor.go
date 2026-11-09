package dto

// One colourway of a SupplierMaterial — the database-backed counterpart to
// saiFabricColors.json, so a newly-added supplier's materials get the same
// swatch picker SAI's hard-coded catalogue already has, without a code
// change per supplier.
type SupplierMaterialColor struct {
	Id                 uint64  `gorm:"primaryKey" json:"id"`
	SupplierMaterialId uint64  `json:"supplier_material_id" gorm:"uniqueIndex:idx_supplier_material_colors_dedupe;not null"`
	Code               string  `json:"code" gorm:"uniqueIndex:idx_supplier_material_colors_dedupe;not null"`
	Category           *string `json:"category"`
	Hex                string  `json:"hex"`
	// Switched off when the supplier no longer has this colour: it stays in
	// the catalogue (old Kas Bon lines still name it) but can't be picked
	// for a new line. Stored as "unavailable" so a new colour, and every
	// existing one, starts available.
	Unavailable      bool             `json:"unavailable" gorm:"not null;default:false"`
	SupplierMaterial SupplierMaterial `json:"-" gorm:"foreignKey:SupplierMaterialId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
