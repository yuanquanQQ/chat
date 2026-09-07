package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddress, DatabaseURL, JWTSecret      string
	InitialAdminUsername, InitialAdminPassword string
	TokenTTL                                   time.Duration
	MaxDevices                                 int
}

func Load() (Config, error) {
	c := Config{
		ListenAddress:        env("LISTEN_ADDRESS", ":8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://chat:change-me@localhost:5432/chat?sslmode=disable"),
		JWTSecret:            env("JWT_SECRET", "development-secret-change-me"),
		InitialAdminUsername: os.Getenv("INITIAL_ADMIN_USERNAME"),
		InitialAdminPassword: os.Getenv("INITIAL_ADMIN_PASSWORD"),
		TokenTTL:             24 * time.Hour,
		MaxDevices:           3,
	}
	if value := os.Getenv("MAX_DEVICES"); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			c.MaxDevices = n
		}
	}
	if len(c.JWTSecret) < 32 || c.JWTSecret == "development-secret-change-me" {
		return c, errors.New("JWT_SECRET must contain at least 32 characters and must not use the development default")
	}
	if c.InitialAdminPassword != "" && (len([]byte(c.InitialAdminPassword)) < 12 || c.InitialAdminPassword == "change-this-before-use") {
		return c, errors.New("initial administrator password must be at least 12 bytes and not a default password")
	}
	if (c.InitialAdminUsername == "") != (c.InitialAdminPassword == "") {
		return c, errors.New("both initial administrator variables must be supplied")
	}
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
