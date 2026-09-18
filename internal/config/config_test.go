package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/rokishi?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8081" || cfg.ShutdownTimeout.String() != "10s" {
		t.Fatalf("unexpected defaults: %+v", cfg)
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
			t.Setenv("SHUTDOWN_TIMEOUT", tc.timeout)
			if _, err := Load(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
