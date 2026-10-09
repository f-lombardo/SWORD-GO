package integrations

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestIntegrationService(t *testing.T) *Service {
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

	return NewService(store)
}

func TestCreateAndUpdateTokenIntegration(t *testing.T) {
	service := newTestIntegrationService(t)

	integration, err := service.Create(context.Background(), CreateInput{
		Name:     "CF Prod",
		Provider: "cloudflare",
		Credentials: Credentials{
			Type:  "api_token",
			Token: "abc123456789",
		},
	})
	if err != nil {
		t.Fatalf("create integration: %v", err)
	}
	if integration.ID == 0 {
		t.Fatalf("expected ID")
	}

	updated, err := service.Update(context.Background(), integration.ID, UpdateInput{
		Name: "CF Production",
		Credentials: Credentials{
			Type: "api_token",
		},
	})
	if err != nil {
		t.Fatalf("update integration: %v", err)
	}
	if updated.Name != "CF Production" {
		t.Fatalf("expected updated name")
	}
	if updated.Credentials.Token != "abc123456789" {
		t.Fatalf("expected existing token to be preserved")
	}
}

func TestMaskCredentials(t *testing.T) {
	masked := MaskCredentials(Credentials{
		Type:  "api_token",
		Token: "1234567890TOKEN",
	})
	if masked.Token == "" || masked.Token == "1234567890TOKEN" {
		t.Fatalf("expected masked token")
	}
}
