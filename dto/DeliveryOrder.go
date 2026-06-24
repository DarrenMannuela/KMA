package dto

type DeliveryOrder struct {
	Id         uint64   `gorm:"primaryKey" json:"id"`
	DeliveryId string   `json:"delivery_id"  default:"01/KMA/SJ/26"`
	ItemName   string   `json:"item_name" default:"apron"`
	Size       *string  `json:"size" default:"S"`
	Amount     int      `json:"amount" default:"1"`
	Delivery   Delivery `json:"-" gorm:"foreignKey:DeliveryId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
