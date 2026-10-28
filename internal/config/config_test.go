package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, key, val string) {
	t.Helper()
	orig, ok := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("failed to set env %s: %v", key, val)
	}
	t.Cleanup(func() {
		if ok {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func clearEnv(t *testing.T, key string) {
	t.Helper()
	_ = os.Unsetenv(key)
}

func TestLoadDefaults(t *testing.T) {
	// Ensure clean environment
	for _, k := range []string{"APP_ENV", "HTTP_ADDR", "HTTP_READ_HEADER_TIMEOUT", "LOG_LEVEL", "MAX_BODY_BYTES"} {
		clearEnv(t, k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("expected development, got %q", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("expected :8080, got %q", cfg.HTTPAddr)
	}
	if cfg.HTTPReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", cfg.HTTPReadHeaderTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected info, got %q", cfg.LogLevel)
	}
	if cfg.MaxBodyBytes != 1048576 {
		t.Errorf("expected 1MB default, got %d", cfg.MaxBodyBytes)
	}
}

func TestLoadCustomValues(t *testing.T) {
	setEnv(t, "APP_ENV", "production")
	setEnv(t, "HTTP_ADDR", ":9090")
	setEnv(t, "HTTP_READ_HEADER_TIMEOUT", "3s")
	setEnv(t, "SHUTDOWN_TIMEOUT", "5s")
	setEnv(t, "MAX_BODY_BYTES", "2097152")
	setEnv(t, "LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("expected production, got %q", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("expected :9090, got %q", cfg.HTTPAddr)
	}
	if cfg.HTTPReadHeaderTimeout != 3*time.Second {
		t.Errorf("expected 3s, got %v", cfg.HTTPReadHeaderTimeout)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", cfg.ShutdownTimeout)
	}
	if cfg.MaxBodyBytes != 2097152 {
		t.Errorf("expected 2097152, got %d", cfg.MaxBodyBytes)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected debug, got %q", cfg.LogLevel)
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"invalid app_env", map[string]string{"APP_ENV": "invalid"}},
		{"empty addr", map[string]string{"HTTP_ADDR": " "}},
		{"invalid timeout", map[string]string{"HTTP_READ_HEADER_TIMEOUT": "abc"}},
		{"invalid max_body", map[string]string{"MAX_BODY_BYTES": "notanumber"}},
		{"negative max_body", map[string]string{"MAX_BODY_BYTES": "-100"}},
		{"invalid log_level", map[string]string{"LOG_LEVEL": "verbose"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Save and set environment
			saved := map[string]string{}
			for k := range tc.env {
				orig, _ := os.LookupEnv(k)
				saved[k] = orig
				_ = os.Setenv(k, tc.env[k])
			}

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// Restore
			for k, v := range saved {
				if v == "" {
					_ = os.Unsetenv(k)
				} else {
					_ = os.Setenv(k, v)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid production", func(t *testing.T) {
		cfg := &Config{
			AppEnv:                "production",
			HTTPAddr:              ":8080",
			LogLevel:              "info",
			HTTPReadHeaderTimeout: 1 * time.Second,
			HTTPReadTimeout:       1 * time.Second,
			HTTPWriteTimeout:      1 * time.Second,
			HTTPIdleTimeout:       1 * time.Second,
			ShutdownTimeout:       1 * time.Second,
			MaxBodyBytes:          1024,
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid log_level", func(t *testing.T) {
		cfg := &Config{
			AppEnv:                "production",
			HTTPAddr:              ":8080",
			LogLevel:              "verbose",
			HTTPReadHeaderTimeout: 1 * time.Second,
			HTTPReadTimeout:       1 * time.Second,
			HTTPWriteTimeout:      1 * time.Second,
			HTTPIdleTimeout:       1 * time.Second,
			ShutdownTimeout:       1 * time.Second,
			MaxBodyBytes:          1024,
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid log_level")
		}
	})
}
