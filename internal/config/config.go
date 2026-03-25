package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all startup parameters (RNF04.1).
// Dynamic config (users, groups, policies, lists) is managed via DB + API.
type Config struct {
	DNS     DNSConfig     `yaml:"dns"`
	Cache   CacheConfig   `yaml:"cache"`
	API     APIConfig     `yaml:"api"`
	Captive CaptiveConfig `yaml:"captive"`
	DB      DBConfig      `yaml:"database"`
	Log     LogConfig     `yaml:"log"`
}

type DNSConfig struct {
	Listen   string   `yaml:"listen"`
	Upstreams []string `yaml:"upstreams"`
	BlockIP  string   `yaml:"block_ip"`
	PortalIP string   `yaml:"portal_ip"`
}

type CacheConfig struct {
	TTLFloorSeconds   int `yaml:"ttl_floor_seconds"`
	TTLCeilingSeconds int `yaml:"ttl_ceiling_seconds"`
}

type APIConfig struct {
	Listen    string `yaml:"listen"`
	JWTSecret string `yaml:"jwt_secret"`
}

type CaptiveConfig struct {
	Listen     string `yaml:"listen"`
	SessionTTL int    `yaml:"session_ttl_hours"`
}

type DBConfig struct {
	URL            string `yaml:"url"`
	RetentionDays  int    `yaml:"retention_days"`
	LogBufferSize  int    `yaml:"log_buffer_size"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// Load reads config from a YAML file.
// Falls back to defaults if file doesn't exist.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Config: %s não encontrado, usando valores padrão\n", path)
			return cfg, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	// Override with environment variables if set
	cfg.applyEnvOverrides()

	fmt.Printf("Config: carregado de %s\n", path)
	return cfg, nil
}

// applyEnvOverrides lets environment variables override YAML values.
// Useful for Docker deployments where you don't want to mount a config file.
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.DB.URL = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.API.JWTSecret = v
	}
	if v := os.Getenv("DNS_LISTEN"); v != "" {
		c.DNS.Listen = v
	}
}