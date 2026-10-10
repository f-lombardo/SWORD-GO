package sites

import (
	"strings"
	"testing"

	"sword-go/internal/app/servers"
)

func TestRenderInstallScriptContainsHardeningAndProvisioningFlow(t *testing.T) {
	script, err := RenderInstallScript(InstallScriptInput{
		Site: Site{
			ID:         99,
			Domain:     "example.test",
			PHPVersion: "8.3",
			DBName:     "db99",
			DBUser:     "user99",
			DBPassword: "pass99",
		},
		Server: servers.Server{
			ID:                10,
			Name:              "srv-10",
			MySQLRootPassword: "mysql-root",
		},
		CallbackURL:      "https://example.test/public/sites/99/callbacks/install?signature=sig",
		AdminUser:        "wpadmin",
		AdminPassword:    "secret-password",
		AdminEmail:       "admin@example.test",
		AdminDisplayName: "Admin Name",
	})
	if err != nil {
		t.Fatalf("render install script: %v", err)
	}

	requiredSnippets := []string{
		"set -Eeuo pipefail",
		`CALLBACK_URL='https://example.test/public/sites/99/callbacks/install?signature=sig'`,
		"PHP_VERSION='8.3'",
		"nonce=\"$(date +%s%N)-$RANDOM\"",
		"CREATE DATABASE IF NOT EXISTS",
		"docker compose -f \"${STACK_DIR}/docker-compose.yml\" up -d --build",
		"--admin_user='wpadmin'",
		"--admin_email='admin@example.test'",
		"updateProgress \"create_database\"",
		"updateProgress \"docker_setup\"",
		"updateProgress \"install_wordpress\"",
		"updateProgress \"installed\" \"installed\"",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("expected install script to contain %q", snippet)
		}
	}
}

func TestRenderInstallScriptRejectsUnsafeDomain(t *testing.T) {
	_, err := RenderInstallScript(InstallScriptInput{
		Site: Site{
			ID:         99,
			Domain:     "example.test;rm -rf /",
			PHPVersion: "8.3",
			DBName:     "db99",
			DBUser:     "user99",
			DBPassword: "pass99",
		},
		Server: servers.Server{
			ID:                10,
			Name:              "srv-10",
			MySQLRootPassword: "mysql-root",
		},
		CallbackURL:      "https://example.test/public/sites/99/callbacks/install?signature=sig",
		AdminUser:        "wpadmin",
		AdminPassword:    "secret-password",
		AdminEmail:       "admin@example.test",
		AdminDisplayName: "Admin Name",
	})
	if err == nil {
		t.Fatalf("expected error for unsafe domain")
	}
}

func TestRenderDeleteScriptContainsCleanupFlow(t *testing.T) {
	script, err := RenderDeleteScript(DeleteScriptInput{
		Site: Site{
			ID:     99,
			Domain: "example.test",
			DBName: "db99",
			DBUser: "user99",
		},
		Server: servers.Server{
			ID:                10,
			Name:              "srv-10",
			MySQLRootPassword: "mysql-root",
		},
	})
	if err != nil {
		t.Fatalf("render delete script: %v", err)
	}

	requiredSnippets := []string{
		"set -Eeuo pipefail",
		"docker compose -f \"${STACK_DIR}/docker-compose.yml\" down --remove-orphans",
		"docker rm -f \"sword_99_php\"",
		"docker rm -f \"sword_99_nginx\"",
		"docker rm -f \"sword_99_redis\"",
		"DROP DATABASE IF EXISTS",
		"DROP USER IF EXISTS '${DB_USER}'@'%'",
		"rm -rf \"${SITE_DIR}\" \"${STACK_DIR}\"",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("expected delete script to contain %q", snippet)
		}
	}
}
