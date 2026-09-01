// Package config loads server configuration from the environment.
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for the expense manager.
type Config struct {
	DBPath        string // path to the SQLite file
	Addr          string // listen address, e.g. 127.0.0.1:8080
	Password      string // the single app password (owner login)
	BearerToken   string // static token accepted on /api/* for non-browser clients
	SessionSecret string // HMAC key for signing session cookies; must be stable across restarts
}

// Environment variable names.
const (
	envDBPath        = "EXPENSE_DB"
	envAddr          = "EXPENSE_ADDR"
	envPassword      = "EXPENSE_PASSWORD"
	envBearerToken   = "EXPENSE_BEARER_TOKEN"
	envSessionSecret = "SESSION_SECRET"
)

// Load reads configuration from the environment. DBPath and Addr have sensible
// defaults; Password, BearerToken, and SessionSecret are required and Load fails
// fast when any is missing, so the app never starts with an open door or a
// per-boot-random signing key that would invalidate sessions on every restart.
func Load() (Config, error) {
	c := Config{
		DBPath:        getenvDefault(envDBPath, "expenses.db"),
		Addr:          getenvDefault(envAddr, "127.0.0.1:8080"),
		Password:      os.Getenv(envPassword),
		BearerToken:   os.Getenv(envBearerToken),
		SessionSecret: os.Getenv(envSessionSecret),
	}

	var missing []string
	if c.Password == "" {
		missing = append(missing, envPassword)
	}
	if c.BearerToken == "" {
		missing = append(missing, envBearerToken)
	}
	if c.SessionSecret == "" {
		missing = append(missing, envSessionSecret)
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return c, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
