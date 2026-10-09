package integrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
CREATE TABLE IF NOT EXISTS integrations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  credentials TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_integrations_provider ON integrations(provider);
CREATE INDEX IF NOT EXISTS idx_integrations_created_at ON integrations(created_at DESC);
`)
	return err
}

func (s *Store) Create(ctx context.Context, integration Integration) (Integration, error) {
	credentialsJSON, err := json.Marshal(integration.Credentials)
	if err != nil {
		return Integration{}, err
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO integrations (name, provider, credentials, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
`,
		integration.Name,
		integration.Provider,
		string(credentialsJSON),
		now,
		now,
	)
	if err != nil {
		return Integration{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Integration{}, err
	}
	return s.GetByID(ctx, id)
}

func (s *Store) Update(ctx context.Context, integration Integration) error {
	credentialsJSON, err := json.Marshal(integration.Credentials)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE integrations
SET name = ?, credentials = ?, updated_at = ?
WHERE id = ?
`,
		integration.Name,
		string(credentialsJSON),
		time.Now().UTC(),
		integration.ID,
	)
	return err
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM integrations WHERE id = ?`, id)
	return err
}

func (s *Store) GetByID(ctx context.Context, id int64) (Integration, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, provider, credentials, created_at, updated_at
FROM integrations
WHERE id = ?
`, id)

	integration, err := scanIntegration(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Integration{}, ErrNotFound
		}
		return Integration{}, err
	}
	return integration, nil
}

func (s *Store) List(ctx context.Context) ([]Integration, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, provider, credentials, created_at, updated_at
FROM integrations
ORDER BY provider ASC, name ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Integration, 0)
	for rows.Next() {
		integration, scanErr := scanIntegration(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, integration)
	}
	return out, rows.Err()
}

func (s *Store) ListByProvider(ctx context.Context, provider string) ([]Integration, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, provider, credentials, created_at, updated_at
FROM integrations
WHERE provider = ?
ORDER BY name ASC
`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Integration, 0)
	for rows.Next() {
		integration, scanErr := scanIntegration(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, integration)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanIntegration(row scanner) (Integration, error) {
	var (
		integration     Integration
		credentialsJSON string
	)
	err := row.Scan(
		&integration.ID,
		&integration.Name,
		&integration.Provider,
		&credentialsJSON,
		&integration.CreatedAt,
		&integration.UpdatedAt,
	)
	if err != nil {
		return Integration{}, err
	}

	if unmarshalErr := json.Unmarshal([]byte(credentialsJSON), &integration.Credentials); unmarshalErr != nil {
		return Integration{}, unmarshalErr
	}

	return integration, nil
}
