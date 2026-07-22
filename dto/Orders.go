package dto

import "time"

type Orders struct {
	Id              string         `json:"id" gorm:"primaryKey;default:01/KB/26"`
	Company         *string        `json:"company" default:"zenbu"`
	PoNumber        *string        `json:"po_number" default:""`
	Date            time.Time      `json:"date" default:""`
	ClientId        *uint64        `json:"client_id"`
	ClientContactId *uint64        `json:"client_contact_id"`
	Client          *Client        `json:"-" gorm:"foreignKey:ClientId;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ClientContact   *ClientContact `json:"-" gorm:"foreignKey:ClientContactId;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
