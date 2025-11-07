package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env         string      `yaml:"env" env-default:"local"`
	StoragePath StoragePath `yaml:"storage_path"`
	HTTPServer  HTTPServer  `yaml:"http_server"`
	HHAuth      HHAuth
	HHRetry     HHRetry `yaml:"hh_retry"`
	Redis       Redis   `yaml:"redis"`
}

type HTTPServer struct {
	Host        string        `yaml:"host" env:"SRV_HOST" env-default:"localhost"`
	Port        string        `yaml:"port" env:"SRV_PORT" env-default:"8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type StoragePath struct {
	Username string `yaml:"username" env:"DB_USERNAME" env-required:"true"`
	Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
	Host     string `yaml:"host" env:"DB_HOST" env-required:"true"`
	Port     int    `yaml:"port" env:"DB_PORT" env-required:"true"`
	Database string `yaml:"database" env:"DB_NAME" env-required:"true"`
	SSLMode  string `yaml:"ssl_mode" env-required:"true"`
}

type Redis struct {
	Addr       string        `yaml:"addr" env:"REDIS_ADDR" env-required:"true"`
	Password   string        `yaml:"password" env:"REDIS_PASSWORD" env-required:"true"`
	DB         int           `yaml:"db" env:"REDIS_DB" env-required:"true"`
	DefaultTTL time.Duration `yaml:"default_ttl" env:"REDIS_DEFAULT_TTL" env-required:"true"`
}

type HHAuth struct {
	ClientID     string `env:"HH_CLIENT_ID" env-required:"true"`
	ClientSecret string `env:"HH_CLIENT_SECRET" env-required:"true"`
	UserAgent    string `env:"HH_USER_AGENT" env-required:"true"`
	AccessToken  string `env:"HH_ACCESS_TOKEN"`
}

type HHRetry struct {
	MaxAttempts  int           `yaml:"max_attempts" env-default:"3"`
	InitialDelay time.Duration `yaml:"initial_delay" env-default:"250ms"`
	MaxDelay     time.Duration `yaml:"max_delay" env-default:"15s"`
	Multiplier   float64       `yaml:"multiplier" env-default:"2.0"`
	MaxTotalTime time.Duration `yaml:"max_total_time" env-default:"45s"`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath)
}

func MustLoadPath(configPath string) *Config {
	// Load environment variables
	// If you call Load without any args it will default to loading .env in the current path.
	// Or godotenv.Load("path/to/custom.env")
	if err := godotenv.Load(); err != nil {
		panic("cannot load .env file: " + err.Error())
	}

	// checking existence config file
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file not found: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}

// fetchConfigPath determines the path to the config file.
//
// Priority:
// 1. Command-line flag --config (example: go run cmd/psa/main.go --config=./config/local.yaml)
// 2. Environment variable CONFIG_PATH
// 3. Default: "./config/local.yaml"
func fetchConfigPath() string {
	var configPath string

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.Parse()

	if configPath != "" {
		return configPath
	}
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		return envPath
	}

	return "./config/local.yaml"
}
