package dto

type Items struct {
	Id       uint64  `gorm:"primaryKey" json:"id"`
	OrderId  string  `json:"order_id"`
	ItemName string  `json:"item_name" default:"apron"`
	Size     *string `json:"size" default:"S"`
	Amount   int     `json:"amount" default:""`
	Price    int64   `json:"price" default:""`
	SubTotal int64   `json:"sub_total" default:""`
	Orders   Orders  `json:"-" gorm:"foreignKey:OrderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
