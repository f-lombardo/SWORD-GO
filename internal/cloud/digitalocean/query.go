package digitalocean

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sword-go/internal/cloud"
)

func (p *Provider) ListAvailableRegions(ctx context.Context, apiKey string) ([]Region, error) {
	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/regions?per_page=200", apiKey)
	if err != nil {
		return nil, err
	}

	var result RegionsResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	var regions []Region

	for _, region := range result.Regions {
		if region.Available {
			regions = append(regions, region)
		}
	}

	return regions, nil
}

func (p *Provider) ListSizes(ctx context.Context, apiKey string) ([]Size, error) {

	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/sizes?per_page=200", apiKey)
	if err != nil {
		return nil, err
	}

	var result SizesResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}
	var sizes []Size

	for _, size := range result.Sizes {
		if size.Available {
			sizes = append(sizes, size)
		}
	}

	return sizes, nil
}

func (region *Region) ListAvailable(sizes []Size) []Size {
	var result []Size

	for _, size := range sizes {
		if slices.Contains(size.Regions, region.Slug) {
			result = append(result, size)
		}
	}

	return result
}

func (size *Size) String() string {
	return fmt.Sprintf("Name: %s, Memory %d MB, VCPUs: %d, Disk %d GB", size.Slug, size.Memory, size.VCPUs, size.Disk)
}

// ListAvailableImages returns all available distribution images.
func (p *Provider) ListAvailableImages(ctx context.Context, apiKey string) ([]Image, error) {
	res, err := p.authorizedRequestWithStatusOK(ctx, http.MethodGet, "/images?type=distribution&per_page=200", apiKey)
	if err != nil {
		return nil, err
	}

	var result ImagesResponse
	if err = cloud.DecodeResponse(res, &result); err != nil {
		return nil, err
	}

	var images []Image
	for _, img := range result.Images {
		if img.Status == "available" || img.Status == "" {
			images = append(images, img)
		}
	}

	return images, nil
}

func (img *Image) String() string {
	ident := img.Slug
	if ident == "" {
		ident = fmt.Sprintf("%d", img.ID)
	}
	return fmt.Sprintf("Slug: %s, Distribution: %s, Name: %s", ident, img.Distribution, img.Name)
}
