package servers

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("server not found")

type ProvisionLogEntry struct {
	Step      string `json:"step"`
	Timestamp string `json:"timestamp"`
}

type Server struct {
	ID                int64
	Name              string
	IPAddress         string
	Hostname          string
	Timezone          string
	Region            string
	Provider          string
	ServerType        string
	Image             string
	SSHPort           int
	SSHPublicKey      string
	SSHPrivateKey     string
	SudoPassword      string
	MySQLRootPassword string
	ProvisionToken    string
	CallbackSignature string
	Status            string
	CurrentStep       string
	ProvisionLog      []ProvisionLogEntry
	ProvisionedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateServerInput struct {
	Name              string
	IPAddress         string
	Hostname          string
	Timezone          string
	Region            string
	Provider          string
	ServerType        string
	Image             string
	SSHPort           int
	SSHPublicKey      string
	SSHPrivateKey     string
	SudoPassword      string
	MySQLRootPassword string
	ProvisionToken    string
	CallbackSignature string
	Status            string
	CurrentStep       string
}
