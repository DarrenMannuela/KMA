package dto

type ClientContact struct {
	Id            uint64  `gorm:"primaryKey" json:"id"`
	ClientId      uint64  `json:"client_id" gorm:"index;not null"`
	Name          string  `json:"name" gorm:"not null"`
	Role          *string `json:"role"`
	PhoneNumber   *string `json:"phone_number"`
	Email         *string `json:"email"`
	LocationLabel *string `json:"location_label"`
	Address       *string `json:"address"`
	IsPrimary     bool    `json:"is_primary" gorm:"default:false"`
	Client        Client  `json:"-" gorm:"foreignKey:ClientId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
