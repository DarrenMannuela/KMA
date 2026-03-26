package database

import (
	"github.com/DarrenMannuela/KMA/dto"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func AutoMigrate() error {
	// Open the database file
	// Note: GORM will create kma.sqlite automatically if it doesn't exist
	db, err := gorm.Open(sqlite.Open("./db_data/kma.sqlite"), &gorm.Config{})
	if err != nil {
		return err
	}

	// This creates the 'suppliers' table based on your Struct in the dto package
	err = db.AutoMigrate(&dto.Supplier{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Operations{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Production{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Orders{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.OrderRecap{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Items{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Delivery{})
	if err != nil {
		return err
	}
	return err
}
