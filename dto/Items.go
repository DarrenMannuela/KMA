package dto

// idx_items_dedupe: a line item is "the same" if it's on the same order,
// same name, same size, and same price — Amount/SubTotal are the only
// things allowed to differ between two "adds" of that combination. This
// index is what PostItems' upsert (see ItemHandler.go) targets, so the
// merge-instead-of-duplicate behavior is enforced by the database itself
// for every caller, not just the frontend form.
//
// Caveat: SQLite treats NULL as distinct from every other NULL in a
// unique index, so two items with size = NULL (no size at all) would NOT
// collide with each other even if name/price match. The frontend avoids
// this by always sending "" instead of null/omitted for a blank size —
// but any other caller (a script, a bulk import) that sends a genuine
// NULL/omitted size will still bypass the dedupe for that row.
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
