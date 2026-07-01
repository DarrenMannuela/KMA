package dto

import "time"

type Delivery struct {
	Id            string    `json:"id" gorm:"primaryKey"`
	Type          string    `json:"type"`
	Address       string    `json:"address"`
	PoNumber      *string   `json:"po_number"`
	PhoneNumber   *string   `json:"phone_number"`
	ContactPerson *string   `json:"contact_person"`
	Date          time.Time `json:"date"`
}
