package dto

type SuratJalan struct {
	Id           uint64   `gorm:"primaryKey" json:"id"`
	DeliveryId   string   `json:"delivery_id"  default:"01/KMA/SJ/26"`
	DeliveryItem string   `json:"delivery_items" default:"Invoice"`
	Delivery     Delivery `json:"-" gorm:"foreignKey:DeliveryId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
