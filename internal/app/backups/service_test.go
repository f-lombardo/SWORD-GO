package backups

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

func newTestBackupService(t *testing.T) (*Service, *Store, *servers.Store, *sites.Store) {
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

	siteStore := sites.NewStore(database)
	if err = siteStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure site schema: %v", err)
	}

	backupStore := NewStore(database)
	if err = backupStore.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("ensure backup schema: %v", err)
	}

	service := NewService(backupStore, serverStore, siteStore)
	service.siteExecutor = fakeSiteBackupExecutor{}
	return service, backupStore, serverStore, siteStore
}

type fakeSiteBackupExecutor struct{}

func (f fakeSiteBackupExecutor) ExecuteSiteBackup(ctx context.Context, schedule Schedule, destination Destination, server servers.Server, site sites.Site) (ExecutionResult, error) {
	size := int64(1024)
	return ExecutionResult{
		Output:      fmt.Sprintf("backup destination=%s site=%s schedule=%d", destination.Name, site.Domain, schedule.ID),
		ArchiveName: site.Domain + "-fake-archive",
		SizeBytes:   &size,
	}, nil
}

func createProvisionedServerForBackups(t *testing.T, store *servers.Store) servers.Server {
	t.Helper()

	server, err := store.Create(context.Background(), servers.CreateServerInput{
		Name:              "Backup Server",
		IPAddress:         "203.0.113.50",
		Hostname:          "backup-server",
		Timezone:          "UTC",
		SSHPort:           22,
		SSHPublicKey:      "ssh-ed25519 AAAA backup",
		SSHPrivateKey:     "invalid-private-key",
		SudoPassword:      "password",
		MySQLRootPassword: "password",
		ProvisionToken:    "token-backup",
		CallbackSignature: "sig-backup",
		Status:            "provisioned",
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	return server
}

func createInstalledSiteForBackups(t *testing.T, store *sites.Store, serverID int64) sites.Site {
	t.Helper()

	site, err := store.Create(context.Background(), sites.CreateSiteInput{
		ServerID:          serverID,
		Domain:            "example.test",
		PHPVersion:        "8.3",
		DBName:            "example_test",
		DBUser:            "example_test",
		DBPassword:        "password",
		InstallToken:      "install-token",
		CallbackSignature: "callback-signature",
		Status:            "installed",
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	return site
}

func createDestinationForBackups(t *testing.T, store *Store) Destination {
	t.Helper()

	destination, err := store.CreateDestination(context.Background(), Destination{
		Name:        "Borg Destination",
		Type:        "borg",
		Host:        "backup.example.test",
		Port:        22,
		Username:    "backup",
		AuthMethod:  "password",
		Password:    "secret",
		StoragePath: "/srv/backups",
		Status:      "connected",
	})
	if err != nil {
		t.Fatalf("create destination: %v", err)
	}

	return destination
}

func TestCreateScheduleRequiresValidFrequency(t *testing.T) {
	service, _, serverStore, _ := newTestBackupService(t)
	server := createProvisionedServerForBackups(t, serverStore)

	_, err := service.CreateSchedule(context.Background(), CreateScheduleInput{
		ServerID:            server.ID,
		BackupDestinationID: 1,
		Frequency:           "hourly",
		Time:                "02:00",
		RetentionCount:      7,
	})
	if err == nil {
		t.Fatalf("expected validation error for invalid frequency")
	}
}

func TestDispatchDueBackupsCreatesRunForInstalledSite(t *testing.T) {
	service, backupStore, serverStore, siteStore := newTestBackupService(t)
	server := createProvisionedServerForBackups(t, serverStore)
	createInstalledSiteForBackups(t, siteStore, server.ID)
	destination := createDestinationForBackups(t, backupStore)

	_, err := service.CreateSchedule(context.Background(), CreateScheduleInput{
		ServerID:            server.ID,
		BackupDestinationID: destination.ID,
		Frequency:           "daily",
		Time:                "02:00",
		RetentionCount:      7,
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	now := time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC)
	dispatched, err := service.DispatchDueBackups(context.Background(), now)
	if err != nil {
		t.Fatalf("dispatch due backups: %v", err)
	}
	if dispatched != 1 {
		t.Fatalf("expected one dispatched schedule, got %d", dispatched)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		runs, listErr := backupStore.ListRunsByServer(context.Background(), server.ID, 10)
		if listErr != nil {
			t.Fatalf("list runs: %v", listErr)
		}
		if len(runs) > 0 {
			if runs[0].Status != "completed" {
				t.Fatalf("expected completed run, got %s", runs[0].Status)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected at least one backup run")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
