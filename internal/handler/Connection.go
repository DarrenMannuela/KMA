package handler

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("./db_data/kma.sqlite"), &gorm.Config{})
	if err != nil {
		return nil
	}
	return db
}
