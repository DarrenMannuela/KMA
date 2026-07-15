package dto

type FinanceHeader struct {
	Id          string `json:"id" gorm:"primaryKey"`
	Date        string `json:"date"`
	Description string `json:"description"`
}
