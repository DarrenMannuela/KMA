package dto

import "time"

type Orders struct {
	Id       string    `json:"id" gorm:"primaryKey;default:01/KB/26"`
	Company  *string   `json:"company" default:"zenbu"`
	PoNumber *string   `json:"po_number" default:""`
	Date     time.Time `json:"date" default:""`
}
