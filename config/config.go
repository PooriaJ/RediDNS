package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// DNS Server configuration
	DNS struct {
		Port     int    `mapstructure:"port"`
		Address  string `mapstructure:"address"`
		Protocol string `mapstructure:"protocol"`
		// SOA configuration
		SOA struct {
			PrimaryNameserver string `mapstructure:"primary_nameserver"`
			MailAddress       string `mapstructure:"mail_address"`
			Refresh           int    `mapstructure:"refresh"`
			Retry             int    `mapstructure:"retry"`
			Expire            int    `mapstructure:"expire"`
			Minimum           int    `mapstructure:"minimum"`
		} `mapstructure:"soa"`
	}

	// API Server configuration
	API struct {
		Port    int    `mapstructure:"port"`
		Address string `mapstructure:"address"`
	}

	// Redis configuration
	Redis struct {
		Address  string `mapstructure:"address"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
		Cache    struct {
			TTL int `mapstructure:"ttl"` // TTL in seconds, 0 means cache forever (until explicit purge)
		} `mapstructure:"cache"`
	}

	// MariaDB configuration
	MariaDB struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		DBName   string `mapstructure:"dbname"`
	}

	// Logging configuration
	Log struct {
		Level string `mapstructure:"level"`
		File  string `mapstructure:"file"`
	}
}

// LoadConfig loads the configuration from file and environment variables
func LoadConfig() (*Config, error) {
	var config Config

	// Set default values
	setDefaults()

	// Read config file
	viper.SetConfigName("config")   // name of config file (without extension)
	viper.SetConfigType("yaml")     // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")        // look for config in the working directory
	viper.AddConfigPath("./config") // look for config in the config directory

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read in config file
	err := viper.ReadInConfig()
	if err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// DNS Server defaults
	viper.SetDefault("dns.port", 53)
	viper.SetDefault("dns.address", "0.0.0.0")
	viper.SetDefault("dns.protocol", "udp")

	// SOA defaults
	viper.SetDefault("dns.soa.primary_nameserver", "ns1.example.com")
	viper.SetDefault("dns.soa.mail_address", "hostmaster.example.com")
	viper.SetDefault("dns.soa.refresh", 7200)
	viper.SetDefault("dns.soa.retry", 3600)
	viper.SetDefault("dns.soa.expire", 1209600)
	viper.SetDefault("dns.soa.minimum", 180)

	// API Server defaults
	viper.SetDefault("api.port", 8080)
	viper.SetDefault("api.address", "0.0.0.0")

	// Redis defaults
	viper.SetDefault("redis.address", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.cache.ttl", 5) // Default cache TTL: 5 minutes

	// MariaDB defaults
	viper.SetDefault("mariadb.host", "localhost")
	viper.SetDefault("mariadb.port", 3306)
	viper.SetDefault("mariadb.user", "root")
	viper.SetDefault("mariadb.password", "123")
	viper.SetDefault("mariadb.dbname", "dns_server")

	// Logging defaults
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.file", "")
}
