package integrations

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Integration, error) {
	return s.store.List(ctx)
}

func (s *Service) ListByProvider(ctx context.Context, provider string) ([]Integration, error) {
	return s.store.ListByProvider(ctx, provider)
}

func (s *Service) GetByID(ctx context.Context, id int64) (Integration, error) {
	return s.store.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Integration, error) {
	if err := validateCreate(input); err != nil {
		return Integration{}, err
	}

	return s.store.Create(ctx, Integration{
		Name:        strings.TrimSpace(input.Name),
		Provider:    strings.TrimSpace(input.Provider),
		Credentials: input.Credentials,
	})
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (Integration, error) {
	existing, err := s.store.GetByID(ctx, id)
	if err != nil {
		return Integration{}, err
	}

	if strings.TrimSpace(input.Name) == "" {
		return Integration{}, errors.New("name is required")
	}
	if strings.TrimSpace(input.Credentials.Type) == "" {
		return Integration{}, errors.New("type is required")
	}

	credentials := Credentials{
		Type: input.Credentials.Type,
	}
	if input.Credentials.Type == "api_token" {
		credentials.Token = strings.TrimSpace(input.Credentials.Token)
		if credentials.Token == "" {
			credentials.Token = existing.Credentials.Token
		}
	} else {
		credentials.Email = strings.TrimSpace(input.Credentials.Email)
		credentials.Key = strings.TrimSpace(input.Credentials.Key)
		if credentials.Email == "" {
			credentials.Email = existing.Credentials.Email
		}
		if credentials.Key == "" {
			credentials.Key = existing.Credentials.Key
		}
	}

	existing.Name = strings.TrimSpace(input.Name)
	existing.Credentials = credentials
	if err = s.store.Update(ctx, existing); err != nil {
		return Integration{}, err
	}
	return s.store.GetByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.Delete(ctx, id)
}

func MaskCredentials(credentials Credentials) Credentials {
	masked := Credentials{
		Type:  credentials.Type,
		Email: credentials.Email,
	}

	if credentials.Token != "" {
		masked.Token = maskTail(credentials.Token)
	}
	if credentials.Key != "" {
		masked.Key = maskTail(credentials.Key)
	}
	return masked
}

func maskTail(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", 28) + value[len(value)-4:]
}

func validateCreate(input CreateInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}

	provider := strings.TrimSpace(input.Provider)
	switch provider {
	case "cloudflare", "digital_ocean", "hetzner":
	default:
		return errors.New("provider must be cloudflare, digital_ocean, or hetzner")
	}

	credentials := input.Credentials
	if credentials.Type != "api_token" && credentials.Type != "global_key" {
		return errors.New("type must be api_token or global_key")
	}
	if credentials.Type == "api_token" && strings.TrimSpace(credentials.Token) == "" {
		return errors.New("token is required when using api_token")
	}
	if credentials.Type == "global_key" {
		if strings.TrimSpace(credentials.Email) == "" {
			return errors.New("email is required when using global_key")
		}
		if strings.TrimSpace(credentials.Key) == "" {
			return errors.New("key is required when using global_key")
		}
	}
	return nil
}
