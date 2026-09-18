package config

import (
	"errors"
	"net"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		HTTPAddr:    os.Getenv("HTTP_ADDR"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL es obligatoria")
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8081"
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil {
		return Config{}, errors.New("HTTP_ADDR debe tener formato host:puerto")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, errors.New("HTTP_ADDR debe usar un puerto entre 1 y 65535")
	}

	shutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT")
	if shutdownTimeout == "" {
		shutdownTimeout = "10s"
	}
	cfg.ShutdownTimeout, err = time.ParseDuration(shutdownTimeout)
	if err != nil || cfg.ShutdownTimeout <= 0 {
		return Config{}, errors.New("SHUTDOWN_TIMEOUT debe ser una duracion positiva")
	}
	return cfg, nil
}
