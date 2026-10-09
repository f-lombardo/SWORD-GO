package servers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"
)

type Service struct {
	store          *Store
	doCreator      *digitalocean.Creator
	hetznerCreator *hetzner.Creator
}

func NewService(store *Store, doCreator *digitalocean.Creator, hetznerCreator *hetzner.Creator) *Service {
	return &Service{
		store:          store,
		doCreator:      doCreator,
		hetznerCreator: hetznerCreator,
	}
}

type CreateRequest struct {
	Name       string
	Hostname   string
	Timezone   string
	SSHPort    int
	Provider   string
	ServerType string
	Region     string
	Image      string
	IPAddress  string
	APIToken   string
}

func (s *Service) CreateServer(ctx context.Context, request CreateRequest) (Server, error) {
	if err := validateCreateRequest(request); err != nil {
		return Server{}, err
	}

	privatePEM, publicAuthorizedKey, err := generateSSHKeyPair()
	if err != nil {
		return Server{}, err
	}

	token, err := randomHex(32)
	if err != nil {
		return Server{}, err
	}

	callbackSeed, err := randomHex(40)
	if err != nil {
		return Server{}, err
	}

	callbackSignatureRaw := sha256.Sum256([]byte(callbackSeed))

	server, err := s.store.Create(ctx, CreateServerInput{
		Name:              request.Name,
		IPAddress:         request.IPAddress,
		Hostname:          request.Hostname,
		Timezone:          request.Timezone,
		Region:            request.Region,
		Provider:          request.Provider,
		ServerType:        request.ServerType,
		Image:             request.Image,
		SSHPort:           request.SSHPort,
		SSHPublicKey:      publicAuthorizedKey,
		SSHPrivateKey:     privatePEM,
		SudoPassword:      randomPassword(32),
		MySQLRootPassword: randomPassword(32),
		ProvisionToken:    token,
		CallbackSignature: hex.EncodeToString(callbackSignatureRaw[:]),
		Status:            "pending",
	})
	if err != nil {
		return Server{}, err
	}

	if request.Provider != "" && request.APIToken != "" {
		go s.provisionCloudServer(server.ID, request.APIToken)
	}

	return server, nil
}

func validateCreateRequest(request CreateRequest) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.New("name is required")
	}

	if strings.TrimSpace(request.Hostname) == "" {
		return errors.New("hostname is required")
	}

	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`).MatchString(strings.TrimSpace(request.Hostname)) {
		return errors.New("hostname may only contain lowercase letters, numbers, and hyphens")
	}

	if strings.TrimSpace(request.Timezone) == "" {
		return errors.New("timezone is required")
	}

	if _, err := time.LoadLocation(request.Timezone); err != nil {
		return errors.New("please select a valid timezone")
	}

	if request.SSHPort < 1 || request.SSHPort > 65535 {
		return errors.New("SSH port must be between 1 and 65535")
	}

	if strings.TrimSpace(request.Provider) == "" && strings.TrimSpace(request.IPAddress) == "" {
		return errors.New("an IP address is required for custom servers")
	}

	if strings.TrimSpace(request.Provider) != "" && strings.TrimSpace(request.ServerType) == "" {
		return errors.New("a server type is required when using a cloud integration")
	}

	return nil
}

func generateSSHKeyPair() (privatePEM string, publicAuthorizedKey string, err error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}

	block := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateDER,
	})

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", "", err
	}

	comment, err := randomHex(4)
	if err != nil {
		return "", "", err
	}

	return string(block), string(ssh.MarshalAuthorizedKey(sshPublicKey)) + "sword-" + comment, nil
}

func randomHex(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

const passwordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomPassword(length int) string {
	if length <= 0 {
		return ""
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return strings.Repeat("a", length)
	}

	out := make([]byte, length)
	for index, value := range bytes {
		out[index] = passwordCharset[int(value)%len(passwordCharset)]
	}

	return string(out)
}

func (s *Service) ListServers(ctx context.Context) ([]Server, error) {
	return s.store.List(ctx)
}

func (s *Service) GetServer(ctx context.Context, serverID int64) (Server, error) {
	return s.store.GetByID(ctx, serverID)
}

func (s *Service) GetServerByProvisionToken(ctx context.Context, serverID int64, token string) (Server, error) {
	return s.store.GetByProvisionToken(ctx, serverID, token)
}

func (s *Service) MarkProvisionProgress(ctx context.Context, serverID int64, signature string, status string, step string) error {
	server, err := s.store.GetByCallbackSignature(ctx, serverID, signature)
	if err != nil {
		return err
	}

	nextStatus := cmpOr(strings.TrimSpace(status), "provisioning")
	nextStep := strings.TrimSpace(step)

	log := server.ProvisionLog
	if nextStep == "started" {
		log = []ProvisionLogEntry{}
	}

	if nextStep != "" {
		log = append(log, ProvisionLogEntry{
			Step:      nextStep,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}

	server.Status = nextStatus
	server.ProvisionLog = log
	if nextStep != "" {
		server.CurrentStep = nextStep
	}

	if nextStatus == "provisioned" {
		now := time.Now().UTC()
		server.ProvisionedAt = &now
		server.CurrentStep = ""
	}

	return s.store.Update(ctx, server)
}

func cmpOr(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) provisionCloudServer(serverID int64, apiToken string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	server, err := s.store.GetByID(ctx, serverID)
	if err != nil {
		return
	}

	server.Status = "provisioning"
	server.CurrentStep = "creating_cloud_server"
	server.ProvisionLog = append(server.ProvisionLog, ProvisionLogEntry{
		Step:      "creating_cloud_server",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	_ = s.store.Update(ctx, server)

	var publicIP string
	switch server.Provider {
	case "digital_ocean":
		image := cmpOr(server.Image, "ubuntu-24-04-x64")
		result, createErr := s.doCreator.Create(ctx, digitalocean.CreateDropletData{
			APIKey:                   apiToken,
			Name:                     server.Hostname,
			ServerType:               server.ServerType,
			Region:                   server.Region,
			Image:                    image,
			PublicKey:                strings.TrimSpace(server.SSHPublicKey),
			PublicIPPollAttempts:     30,
			PublicIPPollIntervalSecs: 5,
		})
		if createErr != nil {
			s.failProvision(ctx, server, createErr)
			return
		}

		if result.PublicIP == nil {
			s.failProvision(ctx, server, errors.New("DigitalOcean droplet created but no public IP was assigned"))
			return
		}
		publicIP = *result.PublicIP
	case "hetzner":
		image := cmpOr(server.Image, "ubuntu-24.04")
		result, createErr := s.hetznerCreator.Create(ctx, hetzner.CreateServerData{
			APIKey:                   apiToken,
			Name:                     server.Hostname,
			ServerType:               server.ServerType,
			Location:                 server.Region,
			Image:                    image,
			PublicKey:                strings.TrimSpace(server.SSHPublicKey),
			PublicIPPollAttempts:     30,
			PublicIPPollIntervalSecs: 5,
		})
		if createErr != nil {
			s.failProvision(ctx, server, createErr)
			return
		}

		if result.PublicIP == nil {
			s.failProvision(ctx, server, errors.New("Hetzner server created but no public IP was assigned"))
			return
		}
		publicIP = *result.PublicIP
	default:
		s.failProvision(ctx, server, fmt.Errorf("unsupported provider: %s", server.Provider))
		return
	}

	server.IPAddress = publicIP
	server.CurrentStep = "cloud_server_created"
	server.ProvisionLog = append(server.ProvisionLog, ProvisionLogEntry{
		Step:      "cloud_server_created",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	_ = s.store.Update(ctx, server)
}

func (s *Service) failProvision(ctx context.Context, server Server, reason error) {
	server.Status = "failed"
	server.CurrentStep = "failed"
	server.ProvisionLog = append(server.ProvisionLog, ProvisionLogEntry{
		Step:      "failed: " + reason.Error(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	_ = s.store.Update(ctx, server)
}
