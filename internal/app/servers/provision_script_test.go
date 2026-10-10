package servers

import (
	"strings"
	"testing"
)

func TestRenderProvisionScriptContainsHardeningAndCoreSteps(t *testing.T) {
	script, err := RenderProvisionScript(ProvisionScriptInput{
		Server: Server{
			ID:                42,
			Name:              "Prod Node",
			Hostname:          "prod-node",
			Timezone:          "UTC",
			SSHPublicKey:      "ssh-ed25519 AAAATEST sword",
			SudoPassword:      "sudo-pass",
			MySQLRootPassword: "mysql-root-pass",
		},
		CallbackURL: "https://example.test/public/servers/42/callbacks/provision?signature=sig",
	})
	if err != nil {
		t.Fatalf("render provision script: %v", err)
	}

	requiredSnippets := []string{
		"set -Eeuo pipefail",
		`CALLBACK_URL='https://example.test/public/servers/42/callbacks/provision?signature=sig'`,
		"nonce=\"$(date +%s%N)-$RANDOM\"",
		"updateProgress \"os_upgrade\"",
		"updateProgress \"install_packages\"",
		"updateProgress \"docker_setup\"",
		"updateProgress \"shared_services\"",
		"docker compose -f /srv/sword/shared/docker-compose.yml up -d",
		"MYSQL_ROOT_PASSWORD='mysql-root-pass'",
		"SWORD_PUBKEY='ssh-ed25519 AAAATEST sword'",
		"updateProgress \"provisioned\" \"provisioned\"",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(script, snippet) {
			t.Fatalf("expected script to contain %q", snippet)
		}
	}
}

func TestRenderProvisionScriptRejectsRelativeCallbackURL(t *testing.T) {
	_, err := RenderProvisionScript(ProvisionScriptInput{
		Server: Server{
			ID:                1,
			Name:              "node",
			Hostname:          "node-1",
			Timezone:          "UTC",
			SSHPublicKey:      "ssh-ed25519 AAAATEST sword",
			SudoPassword:      "pass",
			MySQLRootPassword: "root",
		},
		CallbackURL: "/public/servers/1/callbacks/provision?signature=sig",
	})
	if err == nil {
		t.Fatalf("expected error for relative callback URL")
	}
}
