package sites

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("site not found")

type InstallLogEntry struct {
	Step      string `json:"step"`
	Timestamp string `json:"timestamp"`
}

type Site struct {
	ID                int64
	ServerID          int64
	SiteLabel         string
	Domain            string
	PHPVersion        string
	DBName            string
	DBUser            string
	DBPassword        string
	InstallToken      string
	CallbackSignature string
	Status            string
	CurrentStep       string
	InstallLog        []InstallLogEntry
	InstalledAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateSiteInput struct {
	ServerID          int64
	SiteLabel         string
	Domain            string
	PHPVersion        string
	DBName            string
	DBUser            string
	DBPassword        string
	InstallToken      string
	CallbackSignature string
	Status            string
	CurrentStep       string
}

type InstallRequest struct {
	AdminUser        string
	AdminPassword    string
	AdminEmail       string
	AdminDisplayName string
}
