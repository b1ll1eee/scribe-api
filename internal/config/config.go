package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server     ServerConfig
	Postgres   PostgresConfig
	Redis      RedisConfig
	ClickHouse ClickHouseConfig
	Firebase   FirebaseConfig
	JWT        JWTConfig
	CORS       CORSConfig
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type PostgresConfig struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ClickHouseConfig struct {
	Addr     string
	Database string
	Username string
	Password string
}

type FirebaseConfig struct {
	CredentialsJSON string
	MockMode        bool
}

type JWTConfig struct {
	Secret         string
	AccessTokenTTL time.Duration
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            getEnv("PORT", "8080"),
			ReadTimeout:     getDuration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			ShutdownTimeout: getDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/scribesdb?sslmode=disable"),
			MaxOpenConns: getInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getInt("DB_MAX_IDLE_CONNS", 10),
			MaxLifetime:  getDuration("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_URL", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		ClickHouse: ClickHouseConfig{
			Addr:     getEnv("CLICKHOUSE_ADDR", "localhost:9000"),
			Database: getEnv("CLICKHOUSE_DB", "scribesdb"),
			Username: getEnv("CLICKHOUSE_USER", "default"),
			Password: getEnv("CLICKHOUSE_PASSWORD", ""),
		},
		Firebase: FirebaseConfig{
			CredentialsJSON: getEnv("FIREBASE_CREDENTIALS", ""),
			MockMode:        getBool("FIREBASE_AUTH_MOCK", true),
		},
		JWT: JWTConfig{
			Secret:         getEnv("JWT_SECRET", "scribes-app-super-secret-development-key-change-in-production"),
			AccessTokenTTL: getDuration("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{getEnv("CORS_ORIGIN", "http://localhost:3000")},
		},
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
