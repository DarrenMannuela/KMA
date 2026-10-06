package dto

type OperationItem struct {
	Id          uint          `json:"id" gorm:"primaryKey;autoIncrement"`
	HeaderId    string        `json:"header_id"`
	Category    string        `json:"category"`
	Description string        `json:"description"`
	Price       int64         `json:"price"`
	OrderId     *string       `json:"order_id" gorm:"index"` // the order this cost was for, if any
	Header      FinanceHeader `json:"-" gorm:"foreignKey:HeaderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
