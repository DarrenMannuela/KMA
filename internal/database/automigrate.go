package database

import (
	"github.com/DarrenMannuela/KMA/dto"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func AutoMigrate() error {
	// Open the database file
	// Note: GORM will create kma.sqlite automatically if it doesn't exist
	db, err := gorm.Open(sqlite.Open("./db_data/kma.sqlite?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		return err
	}
	// Close this migration-only connection when done: the server uses its
	// own shared one (handler.Connect), and SQLite only folds the WAL back
	// into the main file once the last connection to it has closed.
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	// This creates the 'suppliers' table based on your Struct in the dto package
	err = db.AutoMigrate(&dto.Supplier{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.FinanceHeader{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.OperationItem{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.ProductionItem{})
	if err != nil {
		return err
	}
	// Client + ClientContact + ClientItem + ClientItemPrice must migrate
	// before Orders — Orders now carries optional client_id/
	// client_contact_id FKs (see Orders.go) that reference these tables.
	err = db.AutoMigrate(&dto.Client{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.ClientContact{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.ClientItem{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.ClientItemPrice{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Orders{})
	if err != nil {
		return err
	}
	err = db.AutoMigrate(&dto.Invoice{})
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
	err = db.AutoMigrate(&dto.DeliveryItem{})
	if err != nil {
		return err
	}
	return err
}
