package dto

// A design a bordir or sablon supplier makes for KMA: a client's logo, a
// slogan, a name tag — what the supplier's "catalogue" really is, since
// they work from photos of designs rather than a list of materials or
// colours. Picked on a Kas Bon line the way a fabric's colour is: the line's
// Bahan becomes the design's label, and its price per piece fills in.
//
// Deduped on (supplier, name, placement, size): the same logo at another
// spot or size is a different job at a different price.
type SupplierDesign struct {
	Id         uint64 `gorm:"primaryKey" json:"id"`
	SupplierId uint64 `json:"supplier_id" gorm:"uniqueIndex:idx_supplier_designs_dedupe;not null"`
	Name       string `json:"name" gorm:"uniqueIndex:idx_supplier_designs_dedupe;not null"`
	// Where it goes on the garment, e.g. "DADA KIRI", "PUNGGUNG".
	Placement *string `json:"placement" gorm:"uniqueIndex:idx_supplier_designs_dedupe"`
	// Its size, e.g. "7 CM", "A4".
	Size *string `json:"size" gorm:"uniqueIndex:idx_supplier_designs_dedupe"`
	// What the supplier charges per piece.
	Price     int64    `json:"price"`
	Notes     *string  `json:"notes"`
	PhotoPath *string  `json:"photo_path"`
	Supplier  Supplier `json:"-" gorm:"foreignKey:SupplierId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
