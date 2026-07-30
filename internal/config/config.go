package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr         string
	DataDir      string
	JWTSecret    string // empty => load/generate from data dir
	MaxBodyBytes int64
	JWTTTLHours  int
}

func Load() Config {
	cfg := Config{
		Addr:         envOr("PHONYG_ADDR", ":8080"),
		DataDir:      envOr("PHONYG_DATA_DIR", "./data"),
		JWTSecret:    strings.TrimSpace(os.Getenv("PHONYG_JWT_SECRET")),
		MaxBodyBytes: envInt64("PHONYG_MAX_BODY_BYTES", 64<<20),
		JWTTTLHours:  envInt("PHONYG_JWT_TTL_HOURS", 24),
	}
	if cfg.JWTTTLHours <= 0 {
		cfg.JWTTTLHours = 24
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 64 << 20
	}
	return cfg
}

func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "phonyg.db")
}

func (c Config) JWTSecretPath() string {
	return filepath.Join(c.DataDir, "jwt_secret")
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envInt64(k string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}
