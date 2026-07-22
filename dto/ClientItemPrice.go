package dto

import "time"

type ClientItemPrice struct {
	Id            uint64     `gorm:"primaryKey" json:"id"`
	ClientItemId  uint64     `json:"client_item_id" gorm:"uniqueIndex:idx_client_item_prices_dedupe;not null"`
	Year          int        `json:"year" gorm:"uniqueIndex:idx_client_item_prices_dedupe;not null"`
	Price         int64      `json:"price"`
	EffectiveDate *time.Time `json:"effective_date"`
	ClientItem    ClientItem `json:"-" gorm:"foreignKey:ClientItemId;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
