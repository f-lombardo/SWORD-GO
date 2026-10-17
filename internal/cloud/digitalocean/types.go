package digitalocean

import "sword-go/internal/cloud"

// CreateDropletData aliases the unified cloud.CreateServerData.
type CreateDropletData = cloud.CreateServerData

// DropletResult aliases the unified cloud.ServerResult.
type DropletResult = cloud.ServerResult

// Pagination links

type RegionsResponse struct {
	Regions []Region `json:"regions"`
	Links   Links    `json:"links"`
}

// Regions

type Region struct {
	Name      string   `json:"name"`
	Slug      string   `json:"slug"`
	Available bool     `json:"available"`
	Features  []string `json:"features"`
}

type Links struct {
	Pages struct {
		Next string `json:"next"`
	} `json:"pages"`
}

// Sizes

type Size struct {
	Slug         string   `json:"slug"`
	Memory       int      `json:"memory"` // MB
	VCPUs        int      `json:"vcpus"`
	Disk         int      `json:"disk"` // GB
	PriceMonthly float64  `json:"price_monthly"`
	PriceHourly  float64  `json:"price_hourly"`
	Regions      []string `json:"regions"`
	Available    bool     `json:"available"`
}

type SizesResponse struct {
	Sizes []Size `json:"sizes"`
	Links Links  `json:"links"`
}
