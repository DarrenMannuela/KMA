package dto

type Production struct {
	Id           string   `json:"id" gorm:"primaryKey;default:01/KB/26"`
	Description  string   `json:"description" default:"Beli bahan"`
	SupplierId   int      `json:"supplier_id" default:""`
	MaterialName string   `json:"material_name" default:"Basic 902"`
	Price        int64    `json:"price" default:"90000"`
	SiUnit       string   `json:"si_unit" default:"yard"`
	Amount       int      `json:"amount" default:"10"`
	Supplier     Supplier `gorm:"foreignKey:SupplierId"`
}
