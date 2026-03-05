package dto

import "time"

type Delivery struct {
	Id            string    `json:"id" gorm:"primaryKey;default:01/KMA/SJ/26"`
	Address       string    `json:"address" default:"hayam wuruk"`
	PoNumber      *string   `json:"po_number" default:"P0000011"`
	PhoneNumber   *string   `json:"phone_number" default:"081219201007"`
	ContactPerson *string   `json:"contact_person" default:"Ibu Tuti"`
	Date          time.Time `json:"date" default:""`
}
