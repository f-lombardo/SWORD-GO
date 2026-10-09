package servers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS servers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  ip_address TEXT,
  hostname TEXT NOT NULL,
  timezone TEXT NOT NULL DEFAULT 'UTC',
  region TEXT,
  provider TEXT,
  server_type TEXT,
  image TEXT,
  ssh_port INTEGER NOT NULL DEFAULT 22,
  ssh_public_key TEXT,
  ssh_private_key TEXT,
  sudo_password TEXT,
  mysql_root_password TEXT,
  provision_token TEXT NOT NULL UNIQUE,
  callback_signature TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'pending',
  current_step TEXT,
  provision_log TEXT,
  provisioned_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_servers_created_at ON servers(created_at DESC);
`)

	return err
}

func (s *Store) Create(ctx context.Context, input CreateServerInput) (Server, error) {
	now := time.Now().UTC()

	logJSON, err := json.Marshal([]ProvisionLogEntry{})
	if err != nil {
		return Server{}, err
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO servers (
  name, ip_address, hostname, timezone, region, provider, server_type, image, ssh_port,
  ssh_public_key, ssh_private_key, sudo_password, mysql_root_password,
  provision_token, callback_signature, status, current_step, provision_log, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		input.Name,
		nullIfEmpty(input.IPAddress),
		input.Hostname,
		input.Timezone,
		nullIfEmpty(input.Region),
		nullIfEmpty(input.Provider),
		nullIfEmpty(input.ServerType),
		nullIfEmpty(input.Image),
		input.SSHPort,
		input.SSHPublicKey,
		input.SSHPrivateKey,
		input.SudoPassword,
		input.MySQLRootPassword,
		input.ProvisionToken,
		input.CallbackSignature,
		input.Status,
		nullIfEmpty(input.CurrentStep),
		string(logJSON),
		now,
		now,
	)
	if err != nil {
		return Server{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Server{}, err
	}

	return s.GetByID(ctx, id)
}

func (s *Store) List(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, ip_address, hostname, timezone, region, provider, server_type, image, ssh_port,
       ssh_public_key, ssh_private_key, sudo_password, mysql_root_password, provision_token,
       callback_signature, status, current_step, provision_log, provisioned_at, created_at, updated_at
FROM servers
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Server, 0)
	for rows.Next() {
		server, scanErr := scanServer(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, server)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (Server, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, ip_address, hostname, timezone, region, provider, server_type, image, ssh_port,
       ssh_public_key, ssh_private_key, sudo_password, mysql_root_password, provision_token,
       callback_signature, status, current_step, provision_log, provisioned_at, created_at, updated_at
FROM servers
WHERE id = ?
`, id)

	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Server{}, ErrNotFound
		}
		return Server{}, err
	}

	return server, nil
}

func (s *Store) GetByProvisionToken(ctx context.Context, id int64, token string) (Server, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, ip_address, hostname, timezone, region, provider, server_type, image, ssh_port,
       ssh_public_key, ssh_private_key, sudo_password, mysql_root_password, provision_token,
       callback_signature, status, current_step, provision_log, provisioned_at, created_at, updated_at
FROM servers
WHERE id = ? AND provision_token = ?
`, id, token)

	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Server{}, ErrNotFound
		}
		return Server{}, err
	}

	return server, nil
}

func (s *Store) GetByCallbackSignature(ctx context.Context, id int64, signature string) (Server, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, ip_address, hostname, timezone, region, provider, server_type, image, ssh_port,
       ssh_public_key, ssh_private_key, sudo_password, mysql_root_password, provision_token,
       callback_signature, status, current_step, provision_log, provisioned_at, created_at, updated_at
FROM servers
WHERE id = ? AND callback_signature = ?
`, id, signature)

	server, err := scanServer(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Server{}, ErrNotFound
		}
		return Server{}, err
	}

	return server, nil
}

func (s *Store) Update(ctx context.Context, server Server) error {
	logJSON, err := json.Marshal(server.ProvisionLog)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE servers
SET ip_address = ?, status = ?, current_step = ?, provision_log = ?, provisioned_at = ?, updated_at = ?
WHERE id = ?
`,
		nullIfEmpty(server.IPAddress),
		server.Status,
		nullIfEmpty(server.CurrentStep),
		string(logJSON),
		nullTime(server.ProvisionedAt),
		time.Now().UTC(),
		server.ID,
	)
	return err
}

func (s *Store) SetIPAddress(ctx context.Context, serverID int64, ipAddress string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE servers
SET ip_address = ?, updated_at = ?
WHERE id = ?
`,
		ipAddress,
		time.Now().UTC(),
		serverID,
	)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (Server, error) {
	var (
		server          Server
		ipAddress       sql.NullString
		region          sql.NullString
		provider        sql.NullString
		serverType      sql.NullString
		image           sql.NullString
		currentStep     sql.NullString
		provisionLogRaw string
		provisionedAt   sql.NullTime
	)

	err := row.Scan(
		&server.ID,
		&server.Name,
		&ipAddress,
		&server.Hostname,
		&server.Timezone,
		&region,
		&provider,
		&serverType,
		&image,
		&server.SSHPort,
		&server.SSHPublicKey,
		&server.SSHPrivateKey,
		&server.SudoPassword,
		&server.MySQLRootPassword,
		&server.ProvisionToken,
		&server.CallbackSignature,
		&server.Status,
		&currentStep,
		&provisionLogRaw,
		&provisionedAt,
		&server.CreatedAt,
		&server.UpdatedAt,
	)
	if err != nil {
		return Server{}, err
	}

	server.IPAddress = ipAddress.String
	server.Region = region.String
	server.Provider = provider.String
	server.ServerType = serverType.String
	server.Image = image.String
	server.CurrentStep = currentStep.String
	if provisionedAt.Valid {
		server.ProvisionedAt = &provisionedAt.Time
	}

	if stringsTrimmed := strings.TrimSpace(provisionLogRaw); stringsTrimmed != "" {
		if jsonErr := json.Unmarshal([]byte(stringsTrimmed), &server.ProvisionLog); jsonErr != nil {
			return Server{}, fmt.Errorf("invalid provision log JSON for server %d: %w", server.ID, jsonErr)
		}
	}

	return server, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
