package config

// Defaults returns a Config with sensible production defaults.
func Defaults() *Config {
	return &Config{
		DNS: DNSConfig{
			Listen:    "0.0.0.0:53",
			Upstreams: []string{"8.8.8.8:53", "8.8.4.4:53", "1.1.1.1:53"},
			BlockIP:   "0.0.0.0",
			PortalIP:  "0.0.0.0",
		},
		Cache: CacheConfig{
			TTLFloorSeconds:   30,
			TTLCeilingSeconds: 3600,
		},
		API: APIConfig{
			Listen:    ":8081",
			JWTSecret: "",
		},
		Captive: CaptiveConfig{
			Listen:     ":80",
			SessionTTL: 8,
		},
		DB: DBConfig{
			URL:           "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable",
			RetentionDays: 120,
			LogBufferSize: 100000,
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
		},
		Log: LogConfig{
			Level: "info",
		},
	}
}
