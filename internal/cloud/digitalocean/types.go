package digitalocean

type CreateDropletData struct {
	APIKey                   string
	Name                     string
	ServerType               string
	Region                   string
	Image                    string
	PublicKey                string
	PublicIPPollAttempts     int
	PublicIPPollIntervalSecs int
}

type DropletResult struct {
	DropletID    int
	Name         string
	Region       string
	Type         string
	Status       string
	PublicIP     *string
	SSHKeyID     any
	SSHKeyName   *string
	SSHKeyStatus string
}
