package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewSQLiteDB(dbPath string) (*Database, error) {
	// Create directory if not exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
	}

	return &Database{DB: db}, nil
}

func InitSQLiteTables(db *Database, chains []string) error {
	for _, chain := range chains {
		tableName := chain
		if tableName == "" {
			tableName = "ethereum"
		}

		// SQLite uses different syntax for table creation
		createSQL := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				address TEXT PRIMARY KEY,
				contract TEXT NOT NULL,
				abi TEXT,
				balance TEXT DEFAULT '0.000000',
				isopensource INTEGER DEFAULT 0,
				isproxy INTEGER DEFAULT 0,
				implementation TEXT,
				createtime DATETIME DEFAULT CURRENT_TIMESTAMP,
				createblock INTEGER DEFAULT 0,
				txlast DATETIME DEFAULT CURRENT_TIMESTAMP,
				isdecompiled INTEGER DEFAULT 0,
				dedcode TEXT,
				scan_result TEXT,
				scan_time DATETIME
			)
		`, tableName)

		if err := db.DB.Exec(createSQL).Error; err != nil {
			return fmt.Errorf("failed to create table %s: %w", tableName, err)
		}

		log.Printf("✅ Table '%s' ready", tableName)
	}
	return nil
}
