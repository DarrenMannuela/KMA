package dto

type Client struct {
	Id         uint64  `gorm:"primaryKey" json:"id"`
	ClientName string  `json:"client_name" gorm:"not null"`
	Address    *string `json:"address"`
	Notes      *string `json:"notes"`
}
