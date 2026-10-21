package cloud

import (
	"context"
)

// CreateServerData holds the common input required to provision a cloud server.
type CreateServerData struct {
	APIKey                   string
	Name                     string
	ServerType               string
	Region                   string
	Location                 string // Alias for Region used by providers like Hetzner
	Image                    string
	PublicKey                string
	RootPassword             string
	PublicIPPollAttempts     int
	PublicIPPollIntervalSecs int
}

// RegionOrLocation returns Region if specified, otherwise Location.
func (d CreateServerData) RegionOrLocation() string {
	if d.Region != "" {
		return d.Region
	}
	return d.Location
}

// ServerResult standardizes the creation output across cloud providers.
type ServerResult struct {
	ID           int
	DropletID    int // Backward-compatibility alias for DigitalOcean
	ServerID     int // Backward-compatibility alias for Hetzner
	Name         string
	Region       string
	Location     string // Backward-compatibility alias for Hetzner
	Type         string // Backward-compatibility alias for DigitalOcean
	ServerType   string
	Status       string
	PublicIP     *string
	SSHKeyID     any
	SSHKeyName   *string
	SSHKeyStatus string
}

// ServerProvider represents any provider capable of provisioning a cloud server.
type ServerProvider interface {
	CreateServer(ctx context.Context, data CreateServerData) (ServerResult, error)
}
