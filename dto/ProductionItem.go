package dto

type ProductionItem struct {
	Id           uint          `json:"id" gorm:"primaryKey;autoIncrement"`
	HeaderId     string        `json:"header_id"`
	MaterialName string        `json:"material_name"`
	Price        int64         `json:"price"`
	SiUnit       string        `json:"si_unit"`
	Amount       float64       `json:"amount"` // 2.5 meters is a real quantity
	SupplierId   int           `json:"supplier_id"`
	OrderId      *string       `json:"order_id" gorm:"index"` // the order this cost was for, if any
	Header       FinanceHeader `json:"-" gorm:"foreignKey:HeaderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Supplier     Supplier      `json:"-" gorm:"foreignKey:SupplierId"`
}
