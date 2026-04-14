package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func DropAllTables() error {
	db, err := gorm.Open(sqlite.Open("./db_data/kma.sqlite"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Add this!
	})

	// 1. Get all table names existing in the database
	var tableNames []string
	// We exclude internal sqlite tables like sqlite_sequence
	err = db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableNames).Error
	if err != nil {
		return err
	}

	// 2. Disable foreign key checks
	db.Exec("PRAGMA foreign_keys = OFF;")

	// 3. Drop 'em all
	for _, name := range tableNames {
		if err := db.Migrator().DropTable(name); err != nil {
			return err
		}
	}

	// 4. Re-enable foreign key checks
	db.Exec("PRAGMA foreign_keys = ON;")

	return nil
}
