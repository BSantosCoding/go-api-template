// /home/bsant/testing/go-api-template/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"strings" // Import strings package
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"database"`
	CORS   CORSConfig   `mapstructure:"cors"`
	JWT    JWTConfig    `mapstructure:"jwt"`
}

// ServerConfig holds server specific configuration
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// DBConfig holds database specific configuration
type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

// CORSConfig holds CORS specific configuration
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"` // Slice of allowed origin strings
}

// JWTConfig holds JWT specific configuration
type JWTConfig struct {
	Secret           string        `mapstructure:"secret"`
	ExpirationMinutes int           `mapstructure:"expiration_minutes"` // Store as int from config/env
	Expiration       time.Duration `mapstructure:"-"`                  // Calculated duration, ignore during unmarshal
}

// Load configuration from file and environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath("/app")

	// --- Set Default Values ---
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.name", "api_db")
	viper.SetDefault("jwt.secret", "default-insecure-secret-key-change-me!") // !! CHANGE THIS VIA ENV !!
	viper.SetDefault("jwt.expiration_minutes", 60)

	// Default CORS: Allow common local dev origins and maybe wildcard for simple setup
	// For production, this SHOULD be overridden by environment variables.
	viper.SetDefault("cors.allowed_origins", []string{"http://localhost:3000", "http://127.0.0.1:3000"})

	// --- Read Config File (Optional) ---
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found, using defaults and environment variables.")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	// --- Bind Environment Variables ---
	viper.SetEnvPrefix("API")
	viper.AutomaticEnv()
	// Allow environment variable CORS_ALLOWED_ORIGINS to override (comma-separated string)
	viper.BindEnv("cors.allowed_origins", "CORS_ALLOWED_ORIGINS")
	viper.BindEnv("jwt.secret", "API_JWT_SECRET")
	viper.BindEnv("jwt.expiration_minutes", "API_JWT_EXPIRATION_MINUTES")

	// --- Unmarshal Config ---
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// --- Manual Override from Specific Environment Variables (Highest Priority) ---
	// Server & DB overrides... (keep existing ones)
	if portStr := os.Getenv("SERVER_PORT"); portStr != "" { // ...
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = port
		}
	}
	if host := os.Getenv("SERVER_HOST"); host != "" { // ...
		cfg.Server.Host = host
	}
	if host := os.Getenv("DB_HOST"); host != "" { // ...
		cfg.DB.Host = host
	}
	if portStr := os.Getenv("DB_PORT"); portStr != "" { // ...
		if port, err := strconv.Atoi(portStr); err == nil {
			cfg.DB.Port = port
		}
	}
	if user := os.Getenv("DB_USER"); user != "" { // ...
		cfg.DB.User = user
	}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" { // ...
		cfg.DB.Password = pass
	}
	if name := os.Getenv("DB_NAME"); name != "" { // ...
		cfg.DB.Name = name
	}


	// Handle CORS_ALLOWED_ORIGINS env var (comma-separated string -> slice)
	if originsStr := os.Getenv("CORS_ALLOWED_ORIGINS"); originsStr != "" {
		cfg.CORS.AllowedOrigins = strings.Split(originsStr, ",")
		// Trim whitespace from each origin
		for i, origin := range cfg.CORS.AllowedOrigins {
			cfg.CORS.AllowedOrigins[i] = strings.TrimSpace(origin)
		}
	}

	// JWT Overrides (using non-prefixed env vars like DB/Server for consistency)
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.JWT.Secret = secret
	}
	if expStr := os.Getenv("JWT_EXPIRATION_MINUTES"); expStr != "" {
		if exp, err := strconv.Atoi(expStr); err == nil {
			cfg.JWT.ExpirationMinutes = exp
		}
	}

	// --- Calculate derived values ---
	cfg.JWT.Expiration = time.Duration(cfg.JWT.ExpirationMinutes) * time.Minute

	// --- Final Validation ---
	if cfg.JWT.Secret == "default-insecure-secret-key-change-me!" {
		log.Println("WARNING: Using default insecure JWT secret. Set JWT_SECRET environment variable.")
	}
	if cfg.JWT.Secret == "" {
		log.Fatal("FATAL: JWT_SECRET cannot be empty.") // Or return an error
	}

	log.Printf("Configuration loaded: Server Port=%d, DB Host=%s, Allowed Origins=%v",
		cfg.Server.Port, cfg.DB.Host, cfg.CORS.AllowedOrigins) // Updated log

	return &cfg, nil
}
