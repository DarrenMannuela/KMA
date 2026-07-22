package dto

type ClientItem struct {
	Id        uint64  `gorm:"primaryKey" json:"id"`
	ClientId  uint64  `json:"client_id" gorm:"uniqueIndex:idx_client_items_dedupe;not null"`
	ItemName  string  `json:"item_name" gorm:"uniqueIndex:idx_client_items_dedupe;not null"`
	Size      *string `json:"size" gorm:"uniqueIndex:idx_client_items_dedupe"`
	Notes     *string `json:"notes"`
	PhotoPath *string `json:"photo_path"`
	Client    Client  `json:"-" gorm:"foreignKey:ClientId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
