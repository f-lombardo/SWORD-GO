package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAPIKeyPriority(t *testing.T) {
	tempDir := t.TempDir()
	secretPath := filepath.Join(tempDir, "secret")
	if err := os.WriteFile(secretPath, []byte(" secret-value \n"), 0o600); err != nil {
		t.Fatalf("failed writing secret file: %v", err)
	}

	t.Setenv("TEST_TOKEN_ENV", "env-value")

	fromCLI, _ := resolveAPIKey("cli-value", secretPath, "TEST_TOKEN_ENV")
	if fromCLI != "cli-value" {
		t.Fatalf("expected cli value, got %s", fromCLI)
	}

	fromSecret, _ := resolveAPIKey("", secretPath, "TEST_TOKEN_ENV")
	if fromSecret != "secret-value" {
		t.Fatalf("expected secret value, got %s", fromSecret)
	}

	fromEnv, _ := resolveAPIKey("", filepath.Join(tempDir, "missing"), "TEST_TOKEN_ENV")
	if fromEnv != "env-value" {
		t.Fatalf("expected env value, got %s", fromEnv)
	}
}

func TestPrintMainUsageIncludesRegions(t *testing.T) {
	var buf strings.Builder
	printMainUsage(&buf)
	output := buf.String()

	if !strings.Contains(output, "sword-go digitalocean regions [--key=TOKEN]") {
		t.Errorf("expected digitalocean regions in usage, got:\n%s", output)
	}
	if !strings.Contains(output, "sword-go hetzner regions [--key=TOKEN]") {
		t.Errorf("expected hetzner regions in usage, got:\n%s", output)
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), &stdout, &stderr, []string{"--help"})
	if err != nil {
		t.Fatalf("expected nil error on --help, got %v", err)
	}
	if !strings.Contains(stdout.String(), "sword-go digitalocean regions") {
		t.Errorf("expected main usage on --help, got:\n%s", stdout.String())
	}
}

func TestRunDigitalOceanRegionsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), &stdout, &stderr, []string{"digitalocean", "regions", "--help"})
	if err != nil {
		t.Fatalf("expected nil error on subcommand --help, got %v", err)
	}
	if !strings.Contains(stderr.String(), "sword-go digitalocean regions [--key=TOKEN]") {
		t.Errorf("expected digitalocean regions usage on --help, got:\n%s", stderr.String())
	}
}

func TestRunHetznerRegionsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), &stdout, &stderr, []string{"hetzner", "regions", "--help"})
	if err != nil {
		t.Fatalf("expected nil error on subcommand --help, got %v", err)
	}
	if !strings.Contains(stderr.String(), "sword-go hetzner regions [--key=TOKEN]") {
		t.Errorf("expected hetzner regions usage on --help, got:\n%s", stderr.String())
	}
}
