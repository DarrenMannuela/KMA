package handler

import (
	"log"
	"sync"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	dbInstance *gorm.DB
	dbOnce     sync.Once
)

// Connect returns one shared connection. SQLite allows one writer at a time,
// so writes queue in Go instead of colliding as "database is locked":
//   - _journal_mode=WAL: reads don't wait for writes.
//   - _busy_timeout=5000: wait up to 5s for the write lock.
//   - SetMaxOpenConns(1): one connection, requests queue for it.
func Connect() *gorm.DB {
	dbOnce.Do(func() {
		conn, err := gorm.Open(
			sqlite.Open("./db_data/kma.sqlite?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"),
			&gorm.Config{},
		)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}

		sqlDB, err := conn.DB()
		if err != nil {
			log.Fatalf("Failed to get underlying sql.DB: %v", err)
		}
		sqlDB.SetMaxOpenConns(1)

		dbInstance = conn
	})
	return dbInstance
}
