package backups

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
)

type SiteBackupExecutor interface {
	ExecuteSiteBackup(ctx context.Context, schedule Schedule, destination Destination, server servers.Server, site sites.Site) (ExecutionResult, error)
}

type ExecutionResult struct {
	Output      string
	ArchiveName string
	SizeBytes   *int64
}

type executionError struct {
	output string
	err    error
}

func (e executionError) Error() string {
	return e.err.Error()
}

func (e executionError) Unwrap() error {
	return e.err
}

type borgExecutor struct{}

func (e borgExecutor) ExecuteSiteBackup(ctx context.Context, schedule Schedule, destination Destination, server servers.Server, site sites.Site) (ExecutionResult, error) {
	runner, err := newSSHRunner(server)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer runner.Close()

	var output strings.Builder

	ensureResult := runner.Run(ctx, "which borg >/dev/null 2>&1 || sudo apt-get install -y borgbackup sshpass")
	output.WriteString("[ensure_installed]\n")
	output.WriteString(ensureResult.Output + "\n")
	if ensureResult.ExitCode != 0 {
		return ExecutionResult{Output: output.String()}, executionError{
			output: output.String(),
			err:    fmt.Errorf("failed to ensure borg installation (exit=%d)", ensureResult.ExitCode),
		}
	}

	setupResult := e.setup(ctx, runner, destination, server, site)
	output.WriteString("[setup]\n")
	output.WriteString(setupResult.Output + "\n")
	setupSucceeded := setupResult.ExitCode == 0 ||
		strings.Contains(setupResult.Output, "already exists") ||
		strings.Contains(setupResult.Output, "already initialized")
	if !setupSucceeded {
		return ExecutionResult{Output: output.String()}, executionError{
			output: output.String(),
			err:    fmt.Errorf("repo setup failed (exit=%d)", setupResult.ExitCode),
		}
	}

	dumpCommand := e.dumpDatabaseCommand(server, site)
	dumpResult := runner.Run(ctx, dumpCommand)
	output.WriteString("[dump]\n")
	output.WriteString(dumpResult.Output + "\n")
	if dumpResult.ExitCode != 0 {
		return ExecutionResult{Output: output.String()}, executionError{
			output: output.String(),
			err:    fmt.Errorf("database dump failed (exit=%d)", dumpResult.ExitCode),
		}
	}

	backupResult, backupCleanup := e.backupCommand(schedule, destination, server, site)
	backupExecResult := runner.Run(ctx, backupResult)
	if backupCleanup != "" {
		_ = runner.Run(ctx, backupCleanup)
	}
	output.WriteString("[backup]\n")
	output.WriteString(backupExecResult.Output + "\n")
	if backupExecResult.ExitCode >= 2 {
		return ExecutionResult{Output: output.String()}, executionError{
			output: output.String(),
			err:    fmt.Errorf("backup failed (exit=%d)", backupExecResult.ExitCode),
		}
	}

	pruneCommand, pruneCleanup := e.pruneCommand(schedule, destination, server, site)
	pruneResult := runner.Run(ctx, pruneCommand)
	if pruneCleanup != "" {
		_ = runner.Run(ctx, pruneCleanup)
	}
	output.WriteString("[prune]\n")
	output.WriteString(pruneResult.Output + "\n")
	if pruneResult.ExitCode != 0 {
		return ExecutionResult{Output: output.String()}, executionError{
			output: output.String(),
			err:    fmt.Errorf("prune failed (exit=%d)", pruneResult.ExitCode),
		}
	}

	return ExecutionResult{
		Output:      output.String(),
		ArchiveName: parseArchiveName(backupExecResult.Output),
		SizeBytes:   parseSizeBytes(backupExecResult.Output),
	}, nil
}

func (e borgExecutor) setup(ctx context.Context, runner *sshRunner, destination Destination, server servers.Server, site sites.Site) commandResult {
	repo := buildRepoPath(destination, server, site.Domain)
	envPrefix, cleanup := buildBorgEnv(destination)

	parentPath := fmt.Sprintf("%s/sites/%s", strings.TrimRight(destination.StoragePath, "/"), server.Hostname)
	mkdirCommand := buildRemoteCommand(destination, "mkdir -p "+shellEscape(parentPath))
	_ = runner.Run(ctx, mkdirCommand)

	command := envPrefix + "borg init --encryption=none " + shellEscape(repo) + " 2>&1"
	result := runner.Run(ctx, command)
	if cleanup != "" {
		_ = runner.Run(ctx, cleanup)
	}
	return result
}

func (e borgExecutor) dumpDatabaseCommand(server servers.Server, site sites.Site) string {
	password := escapeSingleQuotes(server.MySQLRootPassword)
	dbName := shellEscape(site.DBName)
	dumpDir := "/srv/sword/backups/mysql"
	return "mkdir -p " + dumpDir + " && " +
		"docker exec sword_mysql mysqldump -uroot -p'" + password + "' --single-transaction --routines --triggers " + dbName +
		" > " + dumpDir + "/" + site.DBName + ".sql 2>/dev/null"
}

func (e borgExecutor) backupCommand(schedule Schedule, destination Destination, server servers.Server, site sites.Site) (string, string) {
	repo := buildRepoPath(destination, server, site.Domain)
	envPrefix, cleanup := buildBorgEnv(destination)
	archiveName := site.Domain + "-" + time.Now().UTC().Format("2006-01-02T15:04:05")
	repoArchive := shellEscape(repo + "::" + archiveName)

	paths := []string{
		shellEscape("/srv/sword/sites/" + site.Domain),
		shellEscape("/srv/sword/stacks/" + site.Domain),
		shellEscape("/srv/sword/backups/mysql/" + site.DBName + ".sql"),
	}

	command := envPrefix + "borg create --stats --compression auto,zstd " + repoArchive + " " + strings.Join(paths, " ") + " 2>&1"
	return command, cleanup
}

func (e borgExecutor) pruneCommand(schedule Schedule, destination Destination, server servers.Server, site sites.Site) (string, string) {
	repo := buildRepoPath(destination, server, site.Domain)
	envPrefix, cleanup := buildBorgEnv(destination)
	escapedRepo := shellEscape(repo)
	command := fmt.Sprintf("%sborg prune --keep-last=%d --stats %s 2>&1 && %sborg compact %s 2>&1",
		envPrefix,
		schedule.RetentionCount,
		escapedRepo,
		envPrefix,
		escapedRepo,
	)
	return command, cleanup
}

type sshRunner struct {
	client *ssh.Client
}

type commandResult struct {
	Output   string
	ExitCode int
}

func newSSHRunner(server servers.Server) (*sshRunner, error) {
	if strings.TrimSpace(server.IPAddress) == "" {
		return nil, fmt.Errorf("server IP address is missing")
	}
	privateKey, err := parsePrivateKey(server.SSHPrivateKey)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(privateKey)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.IPAddress, server.SSHPort), config)
	if err != nil {
		return nil, err
	}

	return &sshRunner{client: client}, nil
}

func (r *sshRunner) Close() {
	_ = r.client.Close()
}

func (r *sshRunner) Run(ctx context.Context, command string) commandResult {
	session, err := r.client.NewSession()
	if err != nil {
		return commandResult{Output: err.Error(), ExitCode: 1}
	}
	defer session.Close()

	type runResult struct {
		output   []byte
		runError error
	}
	resultChan := make(chan runResult, 1)

	go func() {
		output, runErr := session.CombinedOutput("bash -lc " + shellEscape(command))
		resultChan <- runResult{output: output, runError: runErr}
	}()

	select {
	case <-ctx.Done():
		return commandResult{Output: "context canceled: " + ctx.Err().Error(), ExitCode: 1}
	case result := <-resultChan:
		exitCode := 0
		if result.runError != nil {
			exitCode = 1
			var exitErr *ssh.ExitError
			if errors.As(result.runError, &exitErr) {
				exitCode = exitErr.ExitStatus()
			}
		}
		return commandResult{
			Output:   string(result.output),
			ExitCode: exitCode,
		}
	}
}

func buildRepoPath(destination Destination, server servers.Server, domain string) string {
	storagePath := strings.TrimRight(destination.StoragePath, "/")
	return fmt.Sprintf(
		"ssh://%s@%s:%d%s/sites/%s/%s",
		destination.Username,
		destination.Host,
		destination.Port,
		storagePath,
		server.Hostname,
		domain,
	)
}

func buildBorgEnv(destination Destination) (string, string) {
	cleanup := ""
	port := destination.Port
	if destination.AuthMethod == "ssh_key" {
		keyPath := "/tmp/sword_borg_key_" + randomHexSuffix()
		escapedKeyPath := shellEscape(keyPath)
		cleanup = "rm -f " + escapedKeyPath
		escapedKey := escapeSingleQuotes(destination.SSHPrivateKey)
		prefix := "echo '" + escapedKey + "' > " + escapedKeyPath + " && chmod 600 " + escapedKeyPath + " && " +
			"BORG_UNKNOWN_UNENCRYPTED_REPO_ACCESS_IS_OK=yes " +
			`BORG_RSH="ssh -i ` + escapedKeyPath + " -p " + strconv.Itoa(port) + ` -o StrictHostKeyChecking=accept-new" `
		return prefix, cleanup
	}
	escapedPassword := escapeSingleQuotes(destination.Password)
	prefix := "BORG_UNKNOWN_UNENCRYPTED_REPO_ACCESS_IS_OK=yes " +
		`BORG_RSH="sshpass -p '` + escapedPassword + `' ssh -p ` + strconv.Itoa(port) + ` -o StrictHostKeyChecking=accept-new" `
	return prefix, cleanup
}

func buildRemoteCommand(destination Destination, remoteCommand string) string {
	port := destination.Port
	host := shellEscape(destination.Username + "@" + destination.Host)
	if destination.AuthMethod == "ssh_key" {
		keyPath := "/tmp/sword_mkdir_key_" + randomHexSuffix()
		escapedKeyPath := shellEscape(keyPath)
		escapedKey := escapeSingleQuotes(destination.SSHPrivateKey)
		return "echo '" + escapedKey + "' > " + escapedKeyPath + " && chmod 600 " + escapedKeyPath + " && " +
			"ssh -i " + escapedKeyPath + " -p " + strconv.Itoa(port) + " -o StrictHostKeyChecking=accept-new " + host + " " + shellEscape(remoteCommand) +
			" 2>&1; rm -f " + escapedKeyPath
	}
	escapedPassword := escapeSingleQuotes(destination.Password)
	return "sshpass -p '" + escapedPassword + "' ssh -p " + strconv.Itoa(port) + " -o StrictHostKeyChecking=accept-new " + host + " " + shellEscape(remoteCommand) + " 2>&1"
}

func randomHexSuffix() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())[:8]
}

func escapeSingleQuotes(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

func parseArchiveName(output string) string {
	re := regexp.MustCompile(`Archive name:\s*(.+)`)
	match := re.FindStringSubmatch(output)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseSizeBytes(output string) *int64 {
	re := regexp.MustCompile(`This archive:\s*([\d.]+)\s+(\w+)`)
	match := re.FindStringSubmatch(output)
	if len(match) < 3 {
		return nil
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	unit := match[2]
	var bytes float64
	switch unit {
	case "kB":
		bytes = value * 1_000
	case "MB":
		bytes = value * 1_000_000
	case "GB":
		bytes = value * 1_000_000_000
	default:
		bytes = value
	}
	out := int64(bytes)
	return &out
}
