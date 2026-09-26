package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/rokishi?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8081" || cfg.ShutdownTimeout.String() != "10s" || len(cfg.AllowedOrigins) != 1 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadUsesHerokuPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/rokishi?sslmode=require")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "45678")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://rokishi.pages.dev, http://localhost:5173/")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":45678" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":45678")
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://rokishi.pages.dev" || cfg.AllowedOrigins[1] != "http://localhost:5173" {
		t.Fatalf("AllowedOrigins = %#v", cfg.AllowedOrigins)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name, databaseURL, addr, timeout string
	}{
		{"missing database URL", "", ":8081", "10s"},
		{"invalid address", "postgres://localhost/db", "8081", "10s"},
		{"invalid port", "postgres://localhost/db", ":99999", "10s"},
		{"invalid timeout", "postgres://localhost/db", ":8081", "0s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tc.databaseURL)
			t.Setenv("HTTP_ADDR", tc.addr)
			t.Setenv("PORT", "")
			t.Setenv("SHUTDOWN_TIMEOUT", tc.timeout)
			t.Setenv("CORS_ALLOWED_ORIGINS", "")
			if _, err := Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestLoadRejectsInvalidCORSOrigin(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("HTTP_ADDR", ":8081")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")
	if _, err := Load(); err == nil {
		t.Fatal("expected CORS configuration error")
	}
}
