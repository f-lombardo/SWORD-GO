package sites

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"sword-go/internal/app/servers"
)

func newTestSiteService(t *testing.T) (*Service, *servers.Store) {
	t.Helper()

	database, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	serverStore := servers.NewStore(database)
	if err = serverStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure server schema: %v", err)
	}

	siteStore := NewStore(database)
	if err = siteStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure site schema: %v", err)
	}

	return NewService(siteStore, serverStore, "http://localhost:8088"), serverStore
}

func createProvisionedServer(t *testing.T, store *servers.Store) servers.Server {
	t.Helper()

	server, err := store.Create(context.Background(), servers.CreateServerInput{
		Name:              "Provisioned Server",
		IPAddress:         "203.0.113.5",
		Hostname:          "provisioned-server",
		Timezone:          "UTC",
		Region:            "nbg1",
		Provider:          "",
		ServerType:        "",
		Image:             "",
		SSHPort:           22,
		SSHPublicKey:      "ssh-ed25519 AAAA test",
		SSHPrivateKey:     "invalid-private-key",
		SudoPassword:      "password",
		MySQLRootPassword: "password",
		ProvisionToken:    "token-123",
		CallbackSignature: "signature-123",
		Status:            "provisioned",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	return server
}

func TestCreateSiteRequiresProvisionedServer(t *testing.T) {
	service, serverStore := newTestSiteService(t)

	_, err := serverStore.Create(context.Background(), servers.CreateServerInput{
		Name:              "Pending Server",
		IPAddress:         "203.0.113.4",
		Hostname:          "pending-server",
		Timezone:          "UTC",
		SSHPort:           22,
		SSHPublicKey:      "ssh-ed25519 AAAA test",
		SSHPrivateKey:     "invalid-private-key",
		SudoPassword:      "password",
		MySQLRootPassword: "password",
		ProvisionToken:    "token-999",
		CallbackSignature: "signature-999",
		Status:            "pending",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	_, err = service.CreateSite(context.Background(), CreateRequest{
		ServerID: 1,
		Domain:   "example.com",
		PHPVer:   "8.3",
		Admin: InstallRequest{
			AdminUser:        "admin",
			AdminPassword:    "password123",
			AdminEmail:       "admin@example.com",
			AdminDisplayName: "Admin",
		},
	})
	if err == nil {
		t.Fatalf("expected error for non-provisioned server")
	}
}

func TestCreateSiteBuildsDBSlugAndTokens(t *testing.T) {
	service, serverStore := newTestSiteService(t)
	server := createProvisionedServer(t, serverStore)

	site, err := service.CreateSite(context.Background(), CreateRequest{
		ServerID: server.ID,
		Domain:   "My-Example.Site",
		PHPVer:   "8.3",
		Admin: InstallRequest{
			AdminUser:        "admin",
			AdminPassword:    "password123",
			AdminEmail:       "admin@example.com",
			AdminDisplayName: "Admin",
		},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	if site.ID == 0 {
		t.Fatalf("expected site id")
	}
	if site.DBName == "" || site.DBUser == "" || site.DBPassword == "" {
		t.Fatalf("expected database credentials")
	}
	if site.InstallToken == "" || site.CallbackSignature == "" {
		t.Fatalf("expected tokens")
	}
}

func TestMarkInstallProgressSetsInstalledAt(t *testing.T) {
	service, serverStore := newTestSiteService(t)
	server := createProvisionedServer(t, serverStore)

	site, err := service.CreateSite(context.Background(), CreateRequest{
		ServerID: server.ID,
		Domain:   "example.com",
		PHPVer:   "8.3",
		Admin: InstallRequest{
			AdminUser:        "admin",
			AdminPassword:    "password123",
			AdminEmail:       "admin@example.com",
			AdminDisplayName: "Admin",
		},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	if err = service.MarkInstallProgress(context.Background(), site.ID, site.CallbackSignature, "installing", "started"); err != nil {
		t.Fatalf("mark started: %v", err)
	}
	if err = service.MarkInstallProgress(context.Background(), site.ID, site.CallbackSignature, "installed", "done"); err != nil {
		t.Fatalf("mark installed: %v", err)
	}

	updated, err := service.GetSite(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("get site: %v", err)
	}

	if updated.Status != "installed" {
		t.Fatalf("expected installed status, got %s", updated.Status)
	}
	if updated.InstalledAt == nil {
		t.Fatalf("expected installed_at to be set")
	}
	if len(updated.InstallLog) < 2 {
		t.Fatalf("expected install log entries")
	}
}
