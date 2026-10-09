package backups

import (
	"context"
	"database/sql"
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
CREATE TABLE IF NOT EXISTS backup_destinations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'borg',
  host TEXT NOT NULL,
  port INTEGER NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  auth_method TEXT NOT NULL,
  password TEXT,
  ssh_private_key TEXT,
  storage_path TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  last_connected_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS backup_schedules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id INTEGER NOT NULL,
  backup_destination_id INTEGER NOT NULL,
  frequency TEXT NOT NULL,
  time TEXT NOT NULL,
  day_of_week INTEGER,
  day_of_month INTEGER,
  retention_count INTEGER NOT NULL DEFAULT 7,
  is_enabled INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS backup_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  backup_schedule_id INTEGER NOT NULL,
  server_id INTEGER NOT NULL,
  site_id INTEGER,
  backup_destination_id INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  output TEXT,
  archive_name TEXT,
  size_bytes INTEGER,
  duration_seconds INTEGER,
  started_at DATETIME,
  completed_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_backup_schedules_server_id ON backup_schedules(server_id);
CREATE INDEX IF NOT EXISTS idx_backup_schedules_destination_id ON backup_schedules(backup_destination_id);
CREATE INDEX IF NOT EXISTS idx_backup_runs_schedule_id ON backup_runs(backup_schedule_id);
CREATE INDEX IF NOT EXISTS idx_backup_runs_status ON backup_runs(status);
CREATE INDEX IF NOT EXISTS idx_backup_runs_created_at ON backup_runs(created_at DESC);
`)

	return err
}

func (s *Store) CreateDestination(ctx context.Context, destination Destination) (Destination, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO backup_destinations (
  name, type, host, port, username, auth_method, password, ssh_private_key, storage_path, status, last_connected_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		destination.Name,
		destination.Type,
		destination.Host,
		destination.Port,
		destination.Username,
		destination.AuthMethod,
		nullIfEmpty(destination.Password),
		nullIfEmpty(destination.SSHPrivateKey),
		destination.StoragePath,
		destination.Status,
		nullTime(destination.LastConnectedAt),
		now,
		now,
	)
	if err != nil {
		return Destination{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Destination{}, err
	}

	return s.GetDestinationByID(ctx, id)
}

func (s *Store) UpdateDestination(ctx context.Context, destination Destination) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE backup_destinations
SET name = ?, type = ?, host = ?, port = ?, username = ?, auth_method = ?, password = ?, ssh_private_key = ?, storage_path = ?, status = ?, last_connected_at = ?, updated_at = ?
WHERE id = ?
`,
		destination.Name,
		destination.Type,
		destination.Host,
		destination.Port,
		destination.Username,
		destination.AuthMethod,
		nullIfEmpty(destination.Password),
		nullIfEmpty(destination.SSHPrivateKey),
		destination.StoragePath,
		destination.Status,
		nullTime(destination.LastConnectedAt),
		time.Now().UTC(),
		destination.ID,
	)
	return err
}

func (s *Store) DeleteDestination(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM backup_destinations WHERE id = ?`, id)
	return err
}

func (s *Store) ListDestinations(ctx context.Context) ([]Destination, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, type, host, port, username, auth_method, password, ssh_private_key, storage_path, status, last_connected_at, created_at, updated_at
FROM backup_destinations
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Destination, 0)
	for rows.Next() {
		destination, scanErr := scanDestination(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, destination)
	}

	return out, rows.Err()
}

func (s *Store) GetDestinationByID(ctx context.Context, id int64) (Destination, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, type, host, port, username, auth_method, password, ssh_private_key, storage_path, status, last_connected_at, created_at, updated_at
FROM backup_destinations
WHERE id = ?
`, id)

	destination, err := scanDestination(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Destination{}, ErrDestinationNotFound
		}
		return Destination{}, err
	}
	return destination, nil
}

func (s *Store) CreateSchedule(ctx context.Context, schedule Schedule) (Schedule, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO backup_schedules (
  server_id, backup_destination_id, frequency, time, day_of_week, day_of_month, retention_count, is_enabled, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		schedule.ServerID,
		schedule.BackupDestinationID,
		schedule.Frequency,
		schedule.Time,
		nullInt(schedule.DayOfWeek),
		nullInt(schedule.DayOfMonth),
		schedule.RetentionCount,
		boolAsInt(schedule.IsEnabled),
		now,
		now,
	)
	if err != nil {
		return Schedule{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Schedule{}, err
	}
	return s.GetScheduleByID(ctx, id)
}

func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM backup_schedules WHERE id = ?`, id)
	return err
}

func (s *Store) ListSchedules(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, server_id, backup_destination_id, frequency, time, day_of_week, day_of_month, retention_count, is_enabled, created_at, updated_at
FROM backup_schedules
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Schedule, 0)
	for rows.Next() {
		schedule, scanErr := scanSchedule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, schedule)
	}
	return out, rows.Err()
}

func (s *Store) ListSchedulesByServer(ctx context.Context, serverID int64) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, server_id, backup_destination_id, frequency, time, day_of_week, day_of_month, retention_count, is_enabled, created_at, updated_at
FROM backup_schedules
WHERE server_id = ?
ORDER BY created_at DESC
`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Schedule, 0)
	for rows.Next() {
		schedule, scanErr := scanSchedule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, schedule)
	}
	return out, rows.Err()
}

func (s *Store) GetScheduleByID(ctx context.Context, id int64) (Schedule, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, server_id, backup_destination_id, frequency, time, day_of_week, day_of_month, retention_count, is_enabled, created_at, updated_at
FROM backup_schedules
WHERE id = ?
`, id)

	schedule, err := scanSchedule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Schedule{}, ErrScheduleNotFound
		}
		return Schedule{}, err
	}
	return schedule, nil
}

func (s *Store) CreateRun(ctx context.Context, run Run) (Run, error) {
	now := time.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = now
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO backup_runs (
  backup_schedule_id, server_id, site_id, backup_destination_id, status, output, archive_name, size_bytes, duration_seconds, started_at, completed_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.BackupScheduleID,
		run.ServerID,
		nullInt64(run.SiteID),
		run.BackupDestinationID,
		run.Status,
		nullIfEmpty(run.Output),
		nullIfEmpty(run.ArchiveName),
		nullInt64Value(run.SizeBytes),
		nullInt64Value(run.DurationSeconds),
		nullTime(run.StartedAt),
		nullTime(run.CompletedAt),
		run.CreatedAt,
		run.UpdatedAt,
	)
	if err != nil {
		return Run{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Run{}, err
	}
	return s.GetRunByID(ctx, id)
}

func (s *Store) UpdateRun(ctx context.Context, run Run) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE backup_runs
SET status = ?, output = ?, archive_name = ?, size_bytes = ?, duration_seconds = ?, started_at = ?, completed_at = ?, updated_at = ?
WHERE id = ?
`,
		run.Status,
		nullIfEmpty(run.Output),
		nullIfEmpty(run.ArchiveName),
		nullInt64Value(run.SizeBytes),
		nullInt64Value(run.DurationSeconds),
		nullTime(run.StartedAt),
		nullTime(run.CompletedAt),
		time.Now().UTC(),
		run.ID,
	)
	return err
}

func (s *Store) ListRunsByServer(ctx context.Context, serverID int64, limit int) ([]Run, error) {
	if limit < 1 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, backup_schedule_id, server_id, site_id, backup_destination_id, status, output, archive_name, size_bytes, duration_seconds, started_at, completed_at, created_at, updated_at
FROM backup_runs
WHERE server_id = ?
ORDER BY created_at DESC
LIMIT ?
`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *Store) HasRunningRunForSchedule(ctx context.Context, scheduleID int64) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM backup_runs
WHERE backup_schedule_id = ? AND status = 'running'
`, scheduleID)

	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) HasCompletedRunBetween(ctx context.Context, scheduleID int64, start time.Time, end time.Time) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM backup_runs
WHERE backup_schedule_id = ? AND status = 'completed' AND created_at BETWEEN ? AND ?
`, scheduleID, start.UTC(), end.UTC())

	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) GetRunByID(ctx context.Context, id int64) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, backup_schedule_id, server_id, site_id, backup_destination_id, status, output, archive_name, size_bytes, duration_seconds, started_at, completed_at, created_at, updated_at
FROM backup_runs
WHERE id = ?
`, id)

	return scanRun(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDestination(row scanner) (Destination, error) {
	var (
		destination     Destination
		password        sql.NullString
		sshPrivateKey   sql.NullString
		lastConnectedAt sql.NullTime
	)
	err := row.Scan(
		&destination.ID,
		&destination.Name,
		&destination.Type,
		&destination.Host,
		&destination.Port,
		&destination.Username,
		&destination.AuthMethod,
		&password,
		&sshPrivateKey,
		&destination.StoragePath,
		&destination.Status,
		&lastConnectedAt,
		&destination.CreatedAt,
		&destination.UpdatedAt,
	)
	if err != nil {
		return Destination{}, err
	}
	destination.Password = password.String
	destination.SSHPrivateKey = sshPrivateKey.String
	if lastConnectedAt.Valid {
		destination.LastConnectedAt = &lastConnectedAt.Time
	}
	return destination, nil
}

func scanSchedule(row scanner) (Schedule, error) {
	var (
		schedule   Schedule
		dayOfWeek  sql.NullInt64
		dayOfMonth sql.NullInt64
		isEnabled  int
	)
	err := row.Scan(
		&schedule.ID,
		&schedule.ServerID,
		&schedule.BackupDestinationID,
		&schedule.Frequency,
		&schedule.Time,
		&dayOfWeek,
		&dayOfMonth,
		&schedule.RetentionCount,
		&isEnabled,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)
	if err != nil {
		return Schedule{}, err
	}
	if dayOfWeek.Valid {
		value := int(dayOfWeek.Int64)
		schedule.DayOfWeek = &value
	}
	if dayOfMonth.Valid {
		value := int(dayOfMonth.Int64)
		schedule.DayOfMonth = &value
	}
	schedule.IsEnabled = isEnabled == 1
	return schedule, nil
}

func scanRun(row scanner) (Run, error) {
	var (
		run             Run
		siteID          sql.NullInt64
		output          sql.NullString
		archiveName     sql.NullString
		sizeBytes       sql.NullInt64
		durationSeconds sql.NullInt64
		startedAt       sql.NullTime
		completedAt     sql.NullTime
	)
	err := row.Scan(
		&run.ID,
		&run.BackupScheduleID,
		&run.ServerID,
		&siteID,
		&run.BackupDestinationID,
		&run.Status,
		&output,
		&archiveName,
		&sizeBytes,
		&durationSeconds,
		&startedAt,
		&completedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return Run{}, err
	}
	if siteID.Valid {
		value := siteID.Int64
		run.SiteID = &value
	}
	run.Output = output.String
	run.ArchiveName = archiveName.String
	if sizeBytes.Valid {
		value := sizeBytes.Int64
		run.SizeBytes = &value
	}
	if durationSeconds.Valid {
		value := durationSeconds.Int64
		run.DurationSeconds = &value
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return run, nil
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

func nullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullInt64Value(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func boolAsInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
