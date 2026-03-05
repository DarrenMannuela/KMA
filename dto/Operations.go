package dto

type Operations struct {
	Id          string `json:"id" gorm:"primaryKey;default:01/KB/26"`
	Description string `json:"description" default:"Beli bahan"`
	Price       int64  `json:"price" default:"90000"`
}
