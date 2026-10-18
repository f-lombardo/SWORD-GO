package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPrintMainUsageIncludesRegionsAndImages(t *testing.T) {
	var buf strings.Builder
	printMainUsage(&buf)
	output := buf.String()

	expectedCommands := []string{
		"sword-go digitalocean regions [--key=TOKEN]",
		"sword-go digitalocean images [--key=TOKEN]",
		"sword-go hetzner regions [--key=TOKEN]",
		"sword-go hetzner images [--key=TOKEN]",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(output, cmd) {
			t.Errorf("expected %q in usage, got:\n%s", cmd, output)
		}
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
	if !strings.Contains(stdout.String(), "sword-go digitalocean images") {
		t.Errorf("expected digitalocean images on --help, got:\n%s", stdout.String())
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

func TestRunDigitalOceanImagesHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), &stdout, &stderr, []string{"digitalocean", "images", "--help"})
	if err != nil {
		t.Fatalf("expected nil error on subcommand --help, got %v", err)
	}
	if !strings.Contains(stderr.String(), "sword-go digitalocean images [--key=TOKEN]") {
		t.Errorf("expected digitalocean images usage on --help, got:\n%s", stderr.String())
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

func TestRunHetznerImagesHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), &stdout, &stderr, []string{"hetzner", "images", "--help"})
	if err != nil {
		t.Fatalf("expected nil error on subcommand --help, got %v", err)
	}
	if !strings.Contains(stderr.String(), "sword-go hetzner images [--key=TOKEN]") {
		t.Errorf("expected hetzner images usage on --help, got:\n%s", stderr.String())
	}
}
