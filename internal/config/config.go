package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL     string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
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
		if port := os.Getenv("PORT"); port != "" {
			cfg.HTTPAddr = ":" + port
		} else {
			cfg.HTTPAddr = ":8081"
		}
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
	cfg.AllowedOrigins, err = parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{"http://localhost:5173"}, nil
	}
	origins := make([]string, 0)
	seen := make(map[string]struct{})
	for _, rawOrigin := range strings.Split(value, ",") {
		origin := strings.TrimSpace(strings.TrimSuffix(rawOrigin, "/"))
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
			parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, errors.New("CORS_ALLOWED_ORIGINS debe contener origenes HTTP validos separados por comas")
		}
		if _, exists := seen[origin]; !exists {
			origins = append(origins, origin)
			seen[origin] = struct{}{}
		}
	}
	return origins, nil
}
