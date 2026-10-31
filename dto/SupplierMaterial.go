package dto

// A material in a supplier's own catalogue, editable from the Suppliers
// page — including SAI's, migrated in from its old hand-written code file.
//
// Deduped on (supplier, name, brand) rather than just (supplier, name):
// one supplier can genuinely sell two differently-branded fabrics under the
// same name (SAI's own catalogue has "Tropical Deluxe" from both Verlando
// and Maryland, and "USA Drill" from both Ventura and Venhouston) — a
// two-column dedupe silently merged those pairs into one row the first
// time this was migrated.
type SupplierMaterial struct {
	Id          uint64   `gorm:"primaryKey" json:"id"`
	SupplierId  uint64   `json:"supplier_id" gorm:"uniqueIndex:idx_supplier_materials_dedupe;not null"`
	Name        string   `json:"name" gorm:"uniqueIndex:idx_supplier_materials_dedupe;not null"`
	Brand       *string  `json:"brand" gorm:"uniqueIndex:idx_supplier_materials_dedupe"`
	Composition *string  `json:"composition"`
	UsedFor     *string  `json:"used_for"`
	WidthCm     *float64 `json:"width_cm"`
	RollYards   *float64 `json:"roll_yards"`
	// The current price per unit — a single live figure, not a year-by-year
	// history the way ClientItemPrice tracks what KMA charges a client:
	// this is what the supplier charges KMA, re-typed in place as it
	// changes rather than kept as a priced timeline.
	Price    int64    `json:"price"`
	Supplier Supplier `json:"-" gorm:"foreignKey:SupplierId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
