package dto

type DeliveryItem struct {
	Id         uint64   `json:"id" gorm:"primaryKey"`
	DeliveryId string   `json:"delivery_id"`
	ItemName   string   `json:"item_name"` // physical item OR document name
	Size       *string  `json:"size"`      // null for SJ documents
	Amount     int      `json:"amount"`
	BoxNumber  *int     `json:"box_number"` // for DO — which box
	Delivery   Delivery `json:"-" gorm:"foreignKey:DeliveryId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
