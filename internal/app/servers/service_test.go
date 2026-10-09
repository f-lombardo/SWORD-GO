package servers

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"
)

func newTestService(t *testing.T) *Service {
	t.Helper()

	database, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := NewStore(database)
	if err = store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	return NewService(
		store,
		digitalocean.NewCreator(nil),
		hetzner.NewCreator(nil),
	)
}

func TestCreateServerCustomRequiresIPAddress(t *testing.T) {
	service := newTestService(t)

	_, err := service.CreateServer(context.Background(), CreateRequest{
		Name:     "My Server",
		Hostname: "my-server",
		Timezone: "UTC",
		SSHPort:  22,
	})
	if err == nil {
		t.Fatalf("expected validation error for missing ip")
	}
}

func TestCreateServerWithProviderRequiresServerType(t *testing.T) {
	service := newTestService(t)

	_, err := service.CreateServer(context.Background(), CreateRequest{
		Name:      "Cloud Server",
		Hostname:  "cloud-server",
		Timezone:  "UTC",
		SSHPort:   22,
		Provider:  "digital_ocean",
		Region:    "nyc1",
		IPAddress: "",
	})
	if err == nil {
		t.Fatalf("expected validation error for missing server_type")
	}
}

func TestCreateServerGeneratesSensitiveFieldsAndTokens(t *testing.T) {
	service := newTestService(t)

	server, err := service.CreateServer(context.Background(), CreateRequest{
		Name:      "Custom Server",
		Hostname:  "custom-server",
		Timezone:  "UTC",
		SSHPort:   22,
		IPAddress: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	if server.ID == 0 {
		t.Fatalf("expected server id")
	}
	if server.ProvisionToken == "" || server.CallbackSignature == "" {
		t.Fatalf("expected tokens to be generated")
	}
	if server.SSHPublicKey == "" || server.SSHPrivateKey == "" {
		t.Fatalf("expected ssh keys to be generated")
	}
	if server.Status != "pending" {
		t.Fatalf("expected status pending, got %s", server.Status)
	}
}

func TestMarkProvisionProgressUpdatesStatusAndTimestamps(t *testing.T) {
	service := newTestService(t)

	server, err := service.CreateServer(context.Background(), CreateRequest{
		Name:      "Custom Server",
		Hostname:  "custom-server",
		Timezone:  "UTC",
		SSHPort:   22,
		IPAddress: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	if err = service.MarkProvisionProgress(context.Background(), server.ID, server.CallbackSignature, "provisioning", "started"); err != nil {
		t.Fatalf("mark started: %v", err)
	}

	if err = service.MarkProvisionProgress(context.Background(), server.ID, server.CallbackSignature, "provisioned", "done"); err != nil {
		t.Fatalf("mark provisioned: %v", err)
	}

	updated, err := service.GetServer(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}

	if updated.Status != "provisioned" {
		t.Fatalf("expected status provisioned, got %s", updated.Status)
	}
	if updated.ProvisionedAt == nil {
		t.Fatalf("expected provisioned_at to be set")
	}
	if len(updated.ProvisionLog) < 2 {
		t.Fatalf("expected log entries to be appended")
	}
}
