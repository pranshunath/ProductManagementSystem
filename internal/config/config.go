package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configurations
type Config struct {
	App   AppConfig
	MySQL MySQLConfig
	JWT   JWTConfig
	Redis RedisConfig
	GRPC  GRPCConfig
}

// AppConfig holds HTTP server configurations
type AppConfig struct {
	Env  string
	Port string
}

// MySQLConfig holds MySQL database connection settings
type MySQLConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN returns the MySQL Data Source Name string
func (c *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// BaseDSN returns the MySQL DSN without specific database name (used for creating DB if missing)
func (c *MySQLConfig) BaseDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port)
}

// JWTConfig holds JWT authentication configuration
type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

// RedisConfig holds Redis cache connection settings
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// Addr returns the Redis host:port address
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port string
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Attempt to load .env file if it exists, but don't fail if it doesn't
	if err := godotenv.Load(); err != nil {
		log.Println("[CONFIG] Notice: .env file not found, using system environment variables and defaults")
	}

	cfg := &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8081"),
		},
		MySQL: MySQLConfig{
			Host:            getEnv("MYSQL_HOST", "127.0.0.1"),
			Port:            getEnv("MYSQL_PORT", "3306"),
			User:            getEnv("MYSQL_USER", "root"),
			Password:        getEnv("MYSQL_PASSWORD", ""),
			Database:        getEnv("MYSQL_DATABASE", "producthub_db"),
			MaxOpenConns:    getEnvAsInt("MYSQL_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvAsDuration("MYSQL_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "producthub-super-secret-jwt-key-for-portfolio-2026"),
			ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "127.0.0.1"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		GRPC: GRPCConfig{
			Port: getEnv("GRPC_PORT", "50051"),
		},
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
