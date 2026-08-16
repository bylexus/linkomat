package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Listen       string
	DataDir      string
	CookieSecure bool
	BehindProxy  bool
	CORSOrigins  []string
	AccessTTL    int
	RefreshTTL   int
}

func LoadConfig() (*Config, error) {
	c := &Config{
		Listen:       getenv("LINKOMAT_LISTEN", ":8080"),
		DataDir:      getenv("LINKOMAT_DATA_DIR", "./data"),
		CookieSecure: getenvBool("LINKOMAT_COOKIE_SECURE", true),
		BehindProxy:  getenvBool("LINKOMAT_BEHIND_PROXY", false),
		CORSOrigins:  splitCSV(os.Getenv("LINKOMAT_CORS_ORIGINS")),
	}

	accessTTL, err := getenvInt("LINKOMAT_ACCESS_TTL_SECONDS", 900)
	if err != nil {
		return nil, fmt.Errorf("LINKOMAT_ACCESS_TTL_SECONDS: %w", err)
	}
	c.AccessTTL = accessTTL

	refreshTTL, err := getenvInt("LINKOMAT_REFRESH_TTL_SECONDS", 60*60*24*30)
	if err != nil {
		return nil, fmt.Errorf("LINKOMAT_REFRESH_TTL_SECONDS: %w", err)
	}
	c.RefreshTTL = refreshTTL

	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (c *Config) CORSEnabled() bool {
	return len(c.CORSOrigins) > 0
}

func (c *Config) ListenAddr() string {
	return c.Listen
}