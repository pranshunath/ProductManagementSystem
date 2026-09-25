package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"producthub/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectMySQL initializes a GORM connection to MySQL with connection pooling
func ConnectMySQL(cfg *config.MySQLConfig) (*gorm.DB, error) {
	// Configure GORM logger based on environment
	gormLogLevel := logger.Warn
	if strings.ToLower(cfg.Host) == "localhost" || cfg.Host == "127.0.0.1" {
		gormLogLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	}

	// First try connecting directly to the specified database
	db, err := gorm.Open(mysql.Open(cfg.DSN()), gormConfig)
	if err != nil {
		// If error indicates the database does not exist (Error 1049), attempt to create it
		if strings.Contains(err.Error(), "1049") || strings.Contains(strings.ToLower(err.Error()), "unknown database") {
			log.Printf("[DATABASE] Database '%s' does not exist. Attempting to create it...", cfg.Database)
			if errCreate := ensureDatabaseExists(cfg); errCreate != nil {
				return nil, fmt.Errorf("failed to auto-create database '%s': %w", cfg.Database, errCreate)
			}
			// Retry connecting after creation
			db, err = gorm.Open(mysql.Open(cfg.DSN()), gormConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to MySQL after database creation: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to MySQL at %s:%s: %w", cfg.Host, cfg.Port, err)
		}
	}

	// Retrieve underlying sql.DB instance to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Apply connection pool settings
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify live connectivity
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("MySQL ping failed: %w", err)
	}

	log.Printf("[DATABASE] Successfully connected to MySQL at %s:%s/%s (Pool: max_open=%d, max_idle=%d, lifetime=%v)",
		cfg.Host, cfg.Port, cfg.Database, cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime)

	return db, nil
}

// ensureDatabaseExists connects to MySQL server without a database name and executes CREATE DATABASE IF NOT EXISTS
func ensureDatabaseExists(cfg *config.MySQLConfig) error {
	baseDB, err := sql.Open("mysql", cfg.BaseDSN())
	if err != nil {
		return err
	}
	defer baseDB.Close()

	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", cfg.Database)
	_, err = baseDB.Exec(query)
	return err
}

// CloseMySQL gracefully closes the database connection pool
func CloseMySQL(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
