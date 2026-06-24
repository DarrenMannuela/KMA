package dto

import "time"

type Invoice struct {
	Id      string `json:"id" gorm:"primaryKey"`
	OrderId string `json:"order_id"`
	Type    string `json:"type"` // "dp" or "pelunasan"

	// Client details at time of invoice
	KepadaYth string  `json:"kepada_yth"`
	Untuk     string  `json:"untuk"`
	Alamat    string  `json:"alamat"`
	Email     *string `json:"email"`
	Telp      *string `json:"telp"`

	// Production info
	StartProduksi *string `json:"start_produksi"`
	LamaProduksi  *string `json:"lama_produksi"`

	// Financials
	Total        int64  `json:"total"`
	DownPayment  *int64 `json:"down_payment"`
	Discount     *int64 `json:"discount"`
	Remaining    int64  `json:"remaining"`
	ArReceivable int64  `json:"ar_receivable"`

	// Dates
	Tanggal  time.Time  `json:"tanggal"`
	DueDate  *time.Time `json:"due_date"`
	PaidDate *time.Time `json:"paid_date"`
	Status   string     `json:"status"` // "unpaid" / "paid"

	Orders Orders `json:"-" gorm:"foreignKey:OrderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
