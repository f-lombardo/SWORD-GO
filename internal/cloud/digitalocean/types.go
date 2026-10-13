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

// Regions

type Region struct {
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Available bool     `json:"available"`
	Features  []string `json:"features"`
}

type RegionsResponse struct {
	Regions []Region `json:"regions"`
	Links   struct {
		Pages struct {
			Next string `json:"next"`
		} `json:"pages"`
	} `json:"links"`
}
