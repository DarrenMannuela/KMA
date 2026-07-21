package dto

import "time"

type Delivery struct {
	Id            string    `json:"id" gorm:"primaryKey"`
	Type          string    `json:"type"`
	Company       *string   `json:"company"` // NAMA on the printed DO/SJ — client company name. For DO this mirrors the linked Order's company; for SJ (no order) it's entered directly.
	Address       string    `json:"address"`
	PoNumber      *string   `json:"po_number"`
	PhoneNumber   *string   `json:"phone_number"`
	ContactPerson *string   `json:"contact_person"`
	Date          time.Time `json:"date"`
	OrderId       *string   `json:"order_id"`
	Orders        Orders    `json:"-" gorm:"foreignKey:OrderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
