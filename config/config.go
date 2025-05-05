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
	Blockchain BlockchainConfig `mapstructure:"blockchain"`
	Redis      RedisConfig     `mapstructure:"redis"`
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
	RefreshExpirationHours int           `mapstructure:"refresh_expiration"`
	RefreshExpiration time.Duration         `mapstructure:"-"`
}

// BlockchainConfig holds blockchain interaction configuration
type BlockchainConfig struct {
	RPCURL          string `mapstructure:"rpc_url"`
	ContractAddress string `mapstructure:"contract_address"`
	ContractABIPath string `mapstructure:"contract_abi_path"`
	Expiration       time.Duration `mapstructure:"-"`                  // Calculated duration, ignore during unmarshal
}

// RedisConfig holds Redis connection details.
type RedisConfig struct {
	Addr     string `mapstructure:"REDIS_ADDR"`     // e.g., "localhost:6379"
	Password string `mapstructure:"REDIS_PASSWORD"` // Empty if no password
	DB       int    `mapstructure:"REDIS_DB"`       // e.g., 0
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
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.secret", "default-insecure-secret-key-change-me!")
	viper.SetDefault("jwt.expiration_minutes", 60)
	viper.SetDefault("jwt.refresh_expiration", "24")

	// Defaults for Blockchain Listener 
	viper.SetDefault("blockchain.rpc_url", "wss://ethereum-sepolia-rpc.publicnode.com") 
	viper.SetDefault("blockchain.contract_address", "0x694AA1769357215DE4FAC081bf1f309aDC325306") // (Sepolia ETH/USD on Chainlink aggregator)
	viper.SetDefault("blockchain.contract_abi_path", "config/abi/AggregatorV3Interface.abi.json") // Random price aggregator for example

	// Default CORS: Allow common local dev origins and maybe wildcard for simple setup
	// For production, this SHOULD be overridden by environment variables.
	viper.SetDefault("cors.allowed_origins", []string{"*"})

	// --- Read Config File (Optional) ---
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("Config file not found, using defaults and environment variables.")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}

	// --- Bind Environment Variables ---
	//viper.SetEnvPrefix("API")
	viper.AutomaticEnv()
	// Allow environment variable CORS_ALLOWED_ORIGINS to override (comma-separated string)
	viper.BindEnv("cors.allowed_origins", "CORS_ALLOWED_ORIGINS")
	viper.BindEnv("jwt.secret", "JWT_SECRET")
	viper.BindEnv("jwt.expiration_minutes", "JWT_EXPIRATION_MINUTES")
	viper.BindEnv("jwt.refresh_expiration", "JWT_REFRESH_EXPIRATION")
	viper.BindEnv("blockchain.rpc_url", "BLOCKCHAIN_RPC_URL")
	viper.BindEnv("blockchain.contract_address", "CONTRACT_ADDRESS")
	viper.BindEnv("blockchain.contract_abi_path", "CONTRACT_ABI_PATH")

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

	// JWT Overrides
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.JWT.Secret = secret
	}
	if expStr := os.Getenv("JWT_EXPIRATION_MINUTES"); expStr != "" {
		if exp, err := strconv.Atoi(expStr); err == nil {
			cfg.JWT.ExpirationMinutes = exp
		}
	}
	if rfrExpStr := os.Getenv("JWT_EXPIRATION_MINUTES"); rfrExpStr != "" {
		if rfrExp, err := strconv.Atoi(rfrExpStr); err == nil {
			cfg.JWT.RefreshExpirationHours = rfrExp
		}
	}

	// Blockchain Overrides
	if rpcURL := os.Getenv("BLOCKCHAIN_RPC_URL"); rpcURL != "" {
		cfg.Blockchain.RPCURL = rpcURL
	}
	if contractAddr := os.Getenv("CONTRACT_ADDRESS"); contractAddr != "" {
		cfg.Blockchain.ContractAddress = contractAddr
	}
	if abiPath := os.Getenv("CONTRACT_ABI_PATH"); abiPath != "" {
		cfg.Blockchain.ContractABIPath = abiPath
	}

	// Redis Overrides
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		cfg.Redis.Addr = redisAddr
	}
	if redisPass := os.Getenv("REDIS_PASSWORD"); redisPass != "" {
		cfg.Redis.Password = redisPass
	}
	if redisDBStr := os.Getenv("REDIS_DB"); redisDBStr != "" {
		if redisDB, err := strconv.Atoi(redisDBStr); err == nil {
			cfg.Redis.DB = redisDB
		}
	}

	// --- Calculate derived values ---
	cfg.JWT.Expiration = time.Duration(cfg.JWT.ExpirationMinutes) * time.Minute
	cfg.JWT.RefreshExpiration = time.Duration(cfg.JWT.RefreshExpirationHours) * time.Hour

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
