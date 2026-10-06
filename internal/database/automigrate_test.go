package database

import (
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// A database made before quantities took decimals (amount was an integer
// column) migrates with its rows and constraints intact, and then keeps 2.5.
func TestOldProductionItemsMigrate(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	os.MkdirAll("db_data", 0o755)

	old, err := gorm.Open(sqlite.Open("./db_data/kma.sqlite?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE `suppliers` (`id` integer PRIMARY KEY AUTOINCREMENT,`supplier_name` text,`supplier_category` text)",
		"CREATE TABLE `finance_headers` (`id` text,`date` text,`description` text,PRIMARY KEY (`id`))",
		"CREATE TABLE `production_items` (`id` integer PRIMARY KEY AUTOINCREMENT,`header_id` text,`material_name` text,`price` integer,`si_unit` text,`amount` integer,`supplier_id` integer,CONSTRAINT `fk_production_items_header` FOREIGN KEY (`header_id`) REFERENCES `finance_headers`(`id`) ON DELETE CASCADE ON UPDATE CASCADE,CONSTRAINT `fk_production_items_supplier` FOREIGN KEY (`supplier_id`) REFERENCES `suppliers`(`id`))",
		"INSERT INTO suppliers (supplier_name, supplier_category) VALUES ('SAI', 'sablon')",
		"INSERT INTO finance_headers VALUES ('01/KB/26', '2026-01-02', 'BAHAN')",
		"INSERT INTO production_items (header_id, material_name, price, si_unit, amount, supplier_id) VALUES ('01/KB/26', 'DRILL', 30000, 'meter', 3, 1)",
	} {
		if err := old.Exec(q).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if db, err := old.DB(); err == nil {
		db.Close()
	}

	if err := AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	db, _ := gorm.Open(sqlite.Open("./db_data/kma.sqlite?_foreign_keys=on"), &gorm.Config{})
	defer func() {
		if s, err := db.DB(); err == nil {
			s.Close()
		}
	}()
	var row struct {
		MaterialName string
		Amount       float64
		OrderId      *string
	}
	if err := db.Raw("SELECT material_name, amount, order_id FROM production_items WHERE id = 1").Scan(&row).Error; err != nil || row.MaterialName != "DRILL" || row.Amount != 3 {
		t.Fatalf("old row after migrating: %+v, %v", row, err)
	}
	if err := db.Exec("INSERT INTO production_items (header_id, material_name, price, si_unit, amount, supplier_id) VALUES ('01/KB/26', 'KAIN', 1, 'meter', 2.5, 1)").Error; err != nil {
		t.Fatal(err)
	}
	db.Raw("SELECT material_name, amount FROM production_items WHERE material_name = 'KAIN'").Scan(&row)
	if row.Amount != 2.5 {
		t.Errorf("2.5 meters came back as %v", row.Amount)
	}
	if err := db.Exec("INSERT INTO production_items (header_id, material_name, price, amount, supplier_id) VALUES ('NOPE', 'X', 1, 1, 1)").Error; err == nil {
		t.Error("the Kas Bon foreign key was lost in the migration")
	}
}
