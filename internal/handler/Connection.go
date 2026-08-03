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

// Connect returns a single shared, pooled connection rather than opening a
// fresh one per call. This matters specifically for SQLite: the file only
// ever allows one writer at a time no matter what the application does,
// so previously — every handler opening its own connection — concurrent
// writes (e.g. two people submitting orders at once) could each grab a
// separate connection and collide on that single write lock, surfacing as
// "database is locked" errors instead of one request cleanly waiting a
// few milliseconds for the other.
//
//   - _journal_mode=WAL lets reads proceed without blocking on writes.
//   - _busy_timeout=5000 tells SQLite to wait (up to 5s) for the write
//     lock instead of failing immediately when it's held elsewhere.
//   - SetMaxOpenConns(1) caps Go's own pool at one connection, so
//     concurrent requests queue in Go (fast, in-memory) rather than
//     racing each other for SQLite's write lock at the driver level.
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
