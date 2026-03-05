package dto

type DeliveryOrder struct {
	Id         uint64   `gorm:"primaryKey"`
	DeliveryId string   `json:"delivery_id"  deafult:"01/KMA/SJ/26"`
	ItemName   string   `json:"item_name" default:"apron"`
	Size       *string  `json:"size" default:"S"`
	Amount     int      `json:"amount" default:"1"`
	Delivery   Delivery `gorm:"foreignKey:DeliveryId"`
}
