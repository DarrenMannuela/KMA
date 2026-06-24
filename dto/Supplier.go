package dto

type Supplier struct {
	Id               uint64 `gorm:"primaryKey" json:"id"`
	SupplierName     string `json:"supplier_name" default:"SAI"`
	SupplierCategory string `json:"supplier_category" default:"merchandise"`
}
