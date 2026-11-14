package dto

import "time"

type Orders struct {
	Id              string    `json:"id" gorm:"primaryKey;default:01/KB/26"`
	Company         *string   `json:"company" default:"zenbu"`
	PoNumber        *string   `json:"po_number" default:""`
	Date            time.Time `json:"date" default:""`
	ClientId        *uint64   `json:"client_id" gorm:"index"`
	ClientContactId *uint64   `json:"client_contact_id" gorm:"index"`
	// Set while the order is cancelled (nil = active), with the reason given,
	// if any. Cancelling also marks its unpaid invoices "cancelled", so they
	// stop counting as money owed; reactivating puts them back to "unpaid".
	CancelledAt   *time.Time     `json:"cancelled_at"`
	CancelReason  *string        `json:"cancel_reason"`
	Client        *Client        `json:"-" gorm:"foreignKey:ClientId;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ClientContact *ClientContact `json:"-" gorm:"foreignKey:ClientContactId;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
