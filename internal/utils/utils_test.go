package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomLowerAlphaNumeric(t *testing.T) {
	l := 10
	randomString := RandomLowerAlphaNumeric(l)

	assert.Len(t, randomString, l)
	assert.Equal(t, strings.ToLower(randomString), randomString)
}

func TestResolveAPIKeyPriority(t *testing.T) {
	tempDir := t.TempDir()
	secretPath := filepath.Join(tempDir, "secret")
	if err := os.WriteFile(secretPath, []byte(" secret-value \n"), 0o600); err != nil {
		t.Fatalf("failed writing secret file: %v", err)
	}

	t.Setenv("TEST_TOKEN_ENV", "env-value")

	fromCLI := ResolveSecret("cli-value", secretPath, "TEST_TOKEN_ENV")
	if fromCLI != "cli-value" {
		t.Fatalf("expected cli value, got %s", fromCLI)
	}

	fromSecret := ResolveSecret("", secretPath, "TEST_TOKEN_ENV")
	if fromSecret != "secret-value" {
		t.Fatalf("expected secret value, got %s", fromSecret)
	}

	fromEnv := ResolveSecret("", filepath.Join(tempDir, "missing"), "TEST_TOKEN_ENV")
	if fromEnv != "env-value" {
		t.Fatalf("expected env value, got %s", fromEnv)
	}
}
