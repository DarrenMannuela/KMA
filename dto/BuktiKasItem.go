package dto

// One line on a Bukti Kas — an amount of cash received, optionally for a
// specific order (so it can be matched up against that order's invoices
// the same way a Kas Bon cost line can).
type BuktiKasItem struct {
	Id          uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	HeaderId    string         `json:"header_id"`
	Description string         `json:"description"`
	Price       int64          `json:"price"`
	OrderId     *string        `json:"order_id" gorm:"index"` // the order this cash was received for, if any
	Header      BuktiKasHeader `json:"-" gorm:"foreignKey:HeaderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
