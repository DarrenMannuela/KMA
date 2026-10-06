package dto

// idx_items_dedupe: an item on the same order with the same name, size and
// price is the same item; PostItems' upsert adds the amounts together.
// SQLite treats NULLs as distinct in a unique index, so a NULL size never
// merges: the frontend always sends "" for no size.
type Items struct {
	Id       uint64  `gorm:"primaryKey" json:"id"`
	OrderId  string  `json:"order_id" gorm:"uniqueIndex:idx_items_dedupe"`
	ItemName string  `json:"item_name" default:"apron" gorm:"uniqueIndex:idx_items_dedupe"`
	Size     *string `json:"size" default:"S" gorm:"uniqueIndex:idx_items_dedupe"`
	Amount   int     `json:"amount" default:""`
	Price    int64   `json:"price" default:"" gorm:"uniqueIndex:idx_items_dedupe"`
	SubTotal int64   `json:"sub_total" default:""`
	Orders   Orders  `json:"-" gorm:"foreignKey:OrderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
