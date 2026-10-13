package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAPIKeyPriority(t *testing.T) {
	tempDir := t.TempDir()
	secretPath := filepath.Join(tempDir, "secret")
	if err := os.WriteFile(secretPath, []byte(" secret-value \n"), 0o600); err != nil {
		t.Fatalf("failed writing secret file: %v", err)
	}

	t.Setenv("TEST_TOKEN_ENV", "env-value")

	fromCLI := resolveAPIKey("cli-value", secretPath, "TEST_TOKEN_ENV")
	if fromCLI != "cli-value" {
		t.Fatalf("expected cli value, got %s", fromCLI)
	}

	fromSecret := resolveAPIKey("", secretPath, "TEST_TOKEN_ENV")
	if fromSecret != "secret-value" {
		t.Fatalf("expected secret value, got %s", fromSecret)
	}

	fromEnv := resolveAPIKey("", filepath.Join(tempDir, "missing"), "TEST_TOKEN_ENV")
	if fromEnv != "env-value" {
		t.Fatalf("expected env value, got %s", fromEnv)
	}
}
