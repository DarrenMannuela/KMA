package dto

type FinanceHeader struct {
	Id          string   `json:"id" gorm:"primaryKey"`
	Type        string   `json:"type" gorm:"index"` // "production" | "operation"
	Date        string   `json:"date"`
	SupplierId  int      `json:"supplier_id"`
	Description string   `json:"description"`
	Supplier    Supplier `json:"supplier,omitempty" gorm:"foreignKey:SupplierId"`
}
