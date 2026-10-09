package backups

import (
	"errors"
	"time"
)

var (
	ErrDestinationNotFound = errors.New("backup destination not found")
	ErrScheduleNotFound    = errors.New("backup schedule not found")
)

type Destination struct {
	ID              int64
	Name            string
	Type            string
	Host            string
	Port            int
	Username        string
	AuthMethod      string
	Password        string
	SSHPrivateKey   string
	StoragePath     string
	Status          string
	LastConnectedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Schedule struct {
	ID                  int64
	ServerID            int64
	BackupDestinationID int64
	Frequency           string
	Time                string
	DayOfWeek           *int
	DayOfMonth          *int
	RetentionCount      int
	IsEnabled           bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Run struct {
	ID                  int64
	BackupScheduleID    int64
	ServerID            int64
	SiteID              *int64
	BackupDestinationID int64
	Status              string
	Output              string
	ArchiveName         string
	SizeBytes           *int64
	DurationSeconds     *int64
	StartedAt           *time.Time
	CompletedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateDestinationInput struct {
	Name          string
	Type          string
	Host          string
	Port          int
	Username      string
	AuthMethod    string
	Password      string
	SSHPrivateKey string
	StoragePath   string
}

type UpdateDestinationInput struct {
	Name          string
	Type          string
	Host          string
	Port          int
	Username      string
	AuthMethod    string
	Password      string
	SSHPrivateKey string
	StoragePath   string
}

type CreateScheduleInput struct {
	ServerID            int64
	BackupDestinationID int64
	Frequency           string
	Time                string
	DayOfWeek           *int
	DayOfMonth          *int
	RetentionCount      int
}
