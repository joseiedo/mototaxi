package main

import (
	"testing"
)

// TestEnvConfig verifies envOr returns defaults and set values correctly.
func TestEnvConfig(t *testing.T) {
	const key = "TEST_ENV_CONFIG_VAR_XYZ"

	// Unset env returns default.
	t.Setenv(key, "")
	if got := envOr(key, "default"); got != "default" {
		t.Errorf("unset env: envOr = %q, want %q", got, "default")
	}

	// Set env returns set value.
	t.Setenv(key, "myvalue")
	if got := envOr(key, "default"); got != "myvalue" {
		t.Errorf("set env: envOr = %q, want %q", got, "myvalue")
	}
}

// TestMustInt verifies mustInt returns the correct integer for a valid string.
// Zero, negative, and non-numeric values call log.Fatalf (os.Exit) and cannot
// be unit-tested via panic/recover; they are validated by code inspection.
func TestMustInt(t *testing.T) {
	// Valid positive integer returns the int.
	got := mustInt("TEST_KEY", "42")
	if got != 42 {
		t.Errorf("mustInt(\"42\") = %d, want 42", got)
	}

	got = mustInt("TEST_KEY", "1")
	if got != 1 {
		t.Errorf("mustInt(\"1\") = %d, want 1", got)
	}
}
