package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "a-secret-long-enough-for-unit-tests-123")
	t.Setenv("INITIAL_ADMIN_USERNAME", "")
	t.Setenv("INITIAL_ADMIN_PASSWORD", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxDevices != 3 {
		t.Fatalf("MaxDevices = %d, want 3", c.MaxDevices)
	}
	if c.ListenAddress != ":8080" {
		t.Fatalf("ListenAddress = %q", c.ListenAddress)
	}
}

func TestLoadRejectsPartialAdmin(t *testing.T) {
	t.Setenv("JWT_SECRET", "a-secret-long-enough-for-unit-tests-123")
	t.Setenv("INITIAL_ADMIN_USERNAME", "admin")
	t.Setenv("INITIAL_ADMIN_PASSWORD", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected partial administrator configuration to fail")
	}
}
