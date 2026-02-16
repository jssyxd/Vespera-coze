package config

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB
}

func NewPostgresDB(cfg *DatabaseConfig) (*Database, error) {
	// Use SSL for Supabase connections
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, sslMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (d *Database) AutoMigrate(chain string) error {
	tableName := chain
	if tableName == "" {
		tableName = "ethereum"
	}

	// Create table if not exists
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			address VARCHAR(42) PRIMARY KEY,
			contract TEXT NOT NULL,
			abi JSONB,
			balance VARCHAR(50) DEFAULT '0.000000',
			isopensource BOOLEAN DEFAULT FALSE,
			isproxy BOOLEAN DEFAULT FALSE,
			implementation VARCHAR(42),
			createtime TIMESTAMP NOT NULL DEFAULT NOW(),
			createblock BIGINT NOT NULL DEFAULT 0,
			txlast TIMESTAMP NOT NULL DEFAULT NOW(),
			isdecompiled BOOLEAN DEFAULT FALSE,
			dedcode TEXT,
			scan_result JSONB,
			scan_time TIMESTAMP
		)
	`, tableName)

	if err := d.DB.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	log.Printf("✅ Table '%s' ready", tableName)
	return nil
}

func (d *Database) GetDB() *gorm.DB {
	return d.DB
}
