package dto

type OrderRecap struct {
	Id           string `json:"id" gorm:"primaryKey;default:01/KMA/26"`
	Total        int    `json:"Total" default:"1"`
	DownPayment  *int64 `json:"down_payment" default:"0"`
	Discount     *int64 `json:"discount" default:"1"`
	Amount       int64  `json:"amount" default:""`
	Remaining    int64  `json:"remaining" default:""`
	ArReceivable int64  `json:"sub_total" default:""`
}
