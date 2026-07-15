package dto

type ProductionItem struct {
	Id           uint          `json:"id" gorm:"primaryKey;autoIncrement"`
	HeaderId     string        `json:"header_id"`
	MaterialName string        `json:"material_name"`
	Price        int64         `json:"price"`
	SiUnit       string        `json:"si_unit"`
	Amount       int           `json:"amount"`
	SupplierId   int           `json:"supplier_id"`
	Header       FinanceHeader `json:"-" gorm:"foreignKey:HeaderId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Supplier     Supplier      `json:"-" gorm:"foreignKey:SupplierId"`
}
