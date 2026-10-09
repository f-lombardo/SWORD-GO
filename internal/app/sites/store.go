package sites

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
CREATE TABLE IF NOT EXISTS sites (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id INTEGER NOT NULL,
  site_label TEXT,
  domain TEXT NOT NULL,
  php_version TEXT NOT NULL DEFAULT '8.3',
  db_name TEXT NOT NULL,
  db_user TEXT NOT NULL,
  db_password TEXT NOT NULL,
  install_token TEXT NOT NULL UNIQUE,
  callback_signature TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'pending',
  current_step TEXT,
  install_log TEXT,
  installed_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sites_server_id ON sites(server_id);
CREATE INDEX IF NOT EXISTS idx_sites_created_at ON sites(created_at DESC);
`)

	return err
}

func (s *Store) Create(ctx context.Context, input CreateSiteInput) (Site, error) {
	now := time.Now().UTC()

	logJSON, err := json.Marshal([]InstallLogEntry{})
	if err != nil {
		return Site{}, err
	}

	result, err := s.db.ExecContext(ctx, `
INSERT INTO sites (
 server_id, site_label, domain, php_version, db_name, db_user, db_password,
 install_token, callback_signature, status, current_step, install_log, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		input.ServerID,
		nullIfEmpty(input.SiteLabel),
		input.Domain,
		input.PHPVersion,
		input.DBName,
		input.DBUser,
		input.DBPassword,
		input.InstallToken,
		input.CallbackSignature,
		input.Status,
		nullIfEmpty(input.CurrentStep),
		string(logJSON),
		now,
		now,
	)
	if err != nil {
		return Site{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Site{}, err
	}

	return s.GetByID(ctx, id)
}

func (s *Store) List(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Site, 0)
	for rows.Next() {
		site, scanErr := scanSite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, site)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) ListByServer(ctx context.Context, serverID int64) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
WHERE server_id = ?
ORDER BY created_at DESC
`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Site, 0)
	for rows.Next() {
		site, scanErr := scanSite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, site)
	}
	return out, rows.Err()
}

func (s *Store) ListInstalledByServer(ctx context.Context, serverID int64) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
WHERE server_id = ? AND status = 'installed'
ORDER BY created_at DESC
`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Site, 0)
	for rows.Next() {
		site, scanErr := scanSite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, site)
	}
	return out, rows.Err()
}

func (s *Store) GetByID(ctx context.Context, id int64) (Site, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
WHERE id = ?
`, id)

	site, err := scanSite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Site{}, ErrNotFound
		}
		return Site{}, err
	}

	return site, nil
}

func (s *Store) GetByInstallToken(ctx context.Context, id int64, token string) (Site, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
WHERE id = ? AND install_token = ?
`, id, token)

	site, err := scanSite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Site{}, ErrNotFound
		}
		return Site{}, err
	}

	return site, nil
}

func (s *Store) GetByCallbackSignature(ctx context.Context, id int64, signature string) (Site, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, server_id, site_label, domain, php_version, db_name, db_user, db_password,
       install_token, callback_signature, status, current_step, install_log,
       installed_at, created_at, updated_at
FROM sites
WHERE id = ? AND callback_signature = ?
`, id, signature)

	site, err := scanSite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Site{}, ErrNotFound
		}
		return Site{}, err
	}

	return site, nil
}

func (s *Store) Update(ctx context.Context, site Site) error {
	logJSON, err := json.Marshal(site.InstallLog)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE sites
SET status = ?, current_step = ?, install_log = ?, installed_at = ?, updated_at = ?
WHERE id = ?
`,
		site.Status,
		nullIfEmpty(site.CurrentStep),
		string(logJSON),
		nullTime(site.InstalledAt),
		time.Now().UTC(),
		site.ID,
	)
	return err
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSite(row scanner) (Site, error) {
	var (
		site          Site
		siteLabel     sql.NullString
		currentStep   sql.NullString
		installLogRaw string
		installedAt   sql.NullTime
	)

	err := row.Scan(
		&site.ID,
		&site.ServerID,
		&siteLabel,
		&site.Domain,
		&site.PHPVersion,
		&site.DBName,
		&site.DBUser,
		&site.DBPassword,
		&site.InstallToken,
		&site.CallbackSignature,
		&site.Status,
		&currentStep,
		&installLogRaw,
		&installedAt,
		&site.CreatedAt,
		&site.UpdatedAt,
	)
	if err != nil {
		return Site{}, err
	}

	site.SiteLabel = siteLabel.String
	site.CurrentStep = currentStep.String
	if installedAt.Valid {
		site.InstalledAt = &installedAt.Time
	}

	if stringsTrimmed := strings.TrimSpace(installLogRaw); stringsTrimmed != "" {
		if jsonErr := json.Unmarshal([]byte(stringsTrimmed), &site.InstallLog); jsonErr != nil {
			return Site{}, fmt.Errorf("invalid install log JSON for site %d: %w", site.ID, jsonErr)
		}
	}

	return site, nil
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
