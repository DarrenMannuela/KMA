package dto

type SuratJalan struct {
	Id            uint64    `gorm:"primaryKey"`
	DeliveryId    int       `json:"delivery_id"  deafult:"01/KMA/SJ/26"`
	DeliveryItems []string  `json:"delivery_items" default:"[apron]"`
	Size          []*string `json:"size" default:"[S]"`
	Amount        int       `json:"amount" default:"1"`
	Delivery      Delivery  `gorm:"foreignKey:DeliveryId"`
}
