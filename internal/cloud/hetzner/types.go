package hetzner

type CreateServerData struct {
	APIKey                   string
	Name                     string
	ServerType               string
	Location                 string
	Image                    string
	PublicKey                string
	PublicIPPollAttempts     int
	PublicIPPollIntervalSecs int
}

type ServerResult struct {
	ServerID     int
	Name         string
	Location     string
	ServerType   string
	Status       string
	PublicIP     *string
	SSHKeyID     any
	SSHKeyName   *string
	SSHKeyStatus string
}
