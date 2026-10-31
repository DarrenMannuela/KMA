package dto

// One colourway of a SupplierMaterial — the database-backed counterpart to
// saiFabricColors.json, so a newly-added supplier's materials get the same
// swatch picker SAI's hard-coded catalogue already has, without a code
// change per supplier.
type SupplierMaterialColor struct {
	Id                 uint64           `gorm:"primaryKey" json:"id"`
	SupplierMaterialId uint64           `json:"supplier_material_id" gorm:"uniqueIndex:idx_supplier_material_colors_dedupe;not null"`
	Code               string           `json:"code" gorm:"uniqueIndex:idx_supplier_material_colors_dedupe;not null"`
	Category           *string          `json:"category"`
	Hex                string           `json:"hex"`
	SupplierMaterial   SupplierMaterial `json:"-" gorm:"foreignKey:SupplierMaterialId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
