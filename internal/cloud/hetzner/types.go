package hetzner

import "sword-go/internal/cloud"

// CreateServerData aliases the unified cloud.CreateServerData.
type CreateServerData = cloud.CreateServerData

// ServerResult aliases the unified cloud.ServerResult.
type ServerResult = cloud.ServerResult

//

type Location struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	City        string `json:"city"`
	Country     string `json:"country"`
	NetworkZone string `json:"network_zone"`
}

type ServerTypeLocation struct {
	Name        string `json:"name"`
	Available   bool   `json:"available"`
	Recommended bool   `json:"recommended"`
}

type Money struct {
	Net   string `json:"net"`
	Gross string `json:"gross"`
}

type Price struct {
	Location     string `json:"location"`
	PriceHourly  Money  `json:"price_hourly"`
	PriceMonthly Money  `json:"price_monthly"`
}

type ServerType struct {
	ID           int                  `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Cores        int                  `json:"cores"`
	Memory       float64              `json:"memory"`
	Disk         int                  `json:"disk"`
	Architecture string               `json:"architecture"`
	CPUType      string               `json:"cpu_type"`
	Deprecated   bool                 `json:"deprecated"`
	Locations    []ServerTypeLocation `json:"locations"`
	Prices       []Price              `json:"prices"`
}

type Pagination struct {
	NextPage *int `json:"next_page"`
}

type Meta struct {
	Pagination Pagination `json:"pagination"`
}

type LocationsResponse struct {
	Locations []Location `json:"locations"`
	Meta      Meta       `json:"meta"`
}

type ServerTypesResponse struct {
	ServerTypes []ServerType `json:"server_types"`
	Meta        Meta         `json:"meta"`
}
