package sites

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"sword-go/internal/app/servers"
)

type Service struct {
	store       *Store
	serverStore *servers.Store
	baseURL     string
}

func NewService(store *Store, serverStore *servers.Store, baseURL string) *Service {
	return &Service{
		store:       store,
		serverStore: serverStore,
		baseURL:     strings.TrimRight(baseURL, "/"),
	}
}

type CreateRequest struct {
	ServerID int64
	Domain   string
	PHPVer   string
	Admin    InstallRequest
}

func (s *Service) CreateSite(ctx context.Context, request CreateRequest) (Site, error) {
	if err := s.validateCreateRequest(ctx, request); err != nil {
		return Site{}, err
	}

	dbSlug := makeDBSlug(request.Domain)

	installToken, err := randomHex(32)
	if err != nil {
		return Site{}, err
	}

	callbackSeed, err := randomHex(40)
	if err != nil {
		return Site{}, err
	}
	callbackSignatureRaw := sha256.Sum256([]byte(callbackSeed))

	site, err := s.store.Create(ctx, CreateSiteInput{
		ServerID:          request.ServerID,
		Domain:            strings.TrimSpace(request.Domain),
		PHPVersion:        request.PHPVer,
		DBName:            dbSlug,
		DBUser:            dbSlug,
		DBPassword:        randomPassword(24),
		InstallToken:      installToken,
		CallbackSignature: hex.EncodeToString(callbackSignatureRaw[:]),
		Status:            "pending",
	})
	if err != nil {
		return Site{}, err
	}

	go s.triggerInstall(site.ID, request.Admin)

	return site, nil
}

func (s *Service) validateCreateRequest(ctx context.Context, request CreateRequest) error {
	if request.ServerID < 1 {
		return errors.New("please select a server")
	}

	server, err := s.serverStore.GetByID(ctx, request.ServerID)
	if err != nil {
		if errors.Is(err, servers.ErrNotFound) {
			return errors.New("the selected server does not exist")
		}
		return err
	}

	if server.Status != "provisioned" {
		return errors.New("the selected server is not provisioned")
	}

	domain := strings.TrimSpace(request.Domain)
	if domain == "" {
		return errors.New("enter a domain name for the site")
	}

	if len(domain) > 255 {
		return errors.New("domain must be at most 255 characters")
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9.-]+$`).MatchString(domain) {
		return errors.New("domain contains invalid characters")
	}

	switch request.PHPVer {
	case "8.1", "8.2", "8.3", "8.4":
	default:
		return errors.New("the selected PHP version is not supported")
	}

	if strings.TrimSpace(request.Admin.AdminUser) == "" {
		return errors.New("admin user is required")
	}
	if len(request.Admin.AdminPassword) < 8 {
		return errors.New("admin password must be at least 8 characters")
	}
	if strings.TrimSpace(request.Admin.AdminEmail) == "" || !strings.Contains(request.Admin.AdminEmail, "@") {
		return errors.New("admin email is required")
	}
	if strings.TrimSpace(request.Admin.AdminDisplayName) == "" {
		return errors.New("admin display name is required")
	}

	return nil
}

func makeDBSlug(domain string) string {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	replaced := regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(normalized, "_")
	if len(replaced) > 48 {
		replaced = replaced[:48]
	}
	if replaced == "" {
		return "site_db"
	}
	return replaced
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

func (s *Service) ListSites(ctx context.Context) ([]Site, error) {
	return s.store.List(ctx)
}

func (s *Service) GetSite(ctx context.Context, siteID int64) (Site, error) {
	return s.store.GetByID(ctx, siteID)
}

func (s *Service) DeleteSite(ctx context.Context, siteID int64) error {
	return s.store.Delete(ctx, siteID)
}

func (s *Service) GetSiteByInstallToken(ctx context.Context, siteID int64, token string) (Site, error) {
	return s.store.GetByInstallToken(ctx, siteID, token)
}

func (s *Service) MarkInstallProgress(ctx context.Context, siteID int64, signature string, status string, step string) error {
	site, err := s.store.GetByCallbackSignature(ctx, siteID, signature)
	if err != nil {
		return err
	}

	nextStatus := strings.TrimSpace(status)
	if nextStatus == "" {
		nextStatus = "installing"
	}
	nextStep := strings.TrimSpace(step)

	log := site.InstallLog
	if nextStep != "" {
		log = append(log, InstallLogEntry{
			Step:      nextStep,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}

	site.Status = nextStatus
	site.InstallLog = log
	if nextStep != "" {
		site.CurrentStep = nextStep
	}

	if nextStatus == "installed" {
		now := time.Now().UTC()
		site.InstalledAt = &now
		site.CurrentStep = ""
	}

	return s.store.Update(ctx, site)
}

func (s *Service) triggerInstall(siteID int64, installRequest InstallRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	site, err := s.store.GetByID(ctx, siteID)
	if err != nil {
		return
	}

	server, err := s.serverStore.GetByID(ctx, site.ServerID)
	if err != nil {
		return
	}

	site.Status = "installing"
	site.CurrentStep = "dispatching_install"
	site.InstallLog = append(site.InstallLog, InstallLogEntry{
		Step:      "dispatching_install",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	_ = s.store.Update(ctx, site)

	installURL := fmt.Sprintf("%s/public/sites/%d/scripts/install?token=%s&wp_admin_user=%s&wp_admin_password=%s&wp_admin_email=%s&wp_admin_display_name=%s",
		s.baseURL,
		site.ID,
		url.QueryEscape(site.InstallToken),
		url.QueryEscape(installRequest.AdminUser),
		url.QueryEscape(installRequest.AdminPassword),
		url.QueryEscape(installRequest.AdminEmail),
		url.QueryEscape(installRequest.AdminDisplayName),
	)

	if err = executeRemoteScript(server, fmt.Sprintf(`wget -qO create-wp-site.sh "%s" && nohup bash create-wp-site.sh > create-wp-site.log 2>&1 < /dev/null & disown`, installURL)); err != nil {
		site.Status = "failed"
		site.CurrentStep = "failed"
		site.InstallLog = append(site.InstallLog, InstallLogEntry{
			Step:      "failed: " + err.Error(),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		_ = s.store.Update(ctx, site)
	}
}

func (s *Service) TriggerDeleteRemote(ctx context.Context, site Site) error {
	server, err := s.serverStore.GetByID(ctx, site.ServerID)
	if err != nil {
		return err
	}

	deleteURL := fmt.Sprintf("%s/public/sites/%d/scripts/delete?token=%s", s.baseURL, site.ID, url.QueryEscape(site.InstallToken))
	return executeRemoteScript(server, fmt.Sprintf(`wget -qO delete-wp-site.sh "%s" && bash delete-wp-site.sh > delete-wp-site.log 2>&1`, deleteURL))
}

func executeRemoteScript(server servers.Server, command string) error {
	if strings.TrimSpace(server.IPAddress) == "" {
		return errors.New("server IP address is missing")
	}

	signer, err := signerFromOpenSSHPrivateKey(server.SSHPrivateKey)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", server.IPAddress, server.SSHPort), config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return session.Run(command)
}

func signerFromOpenSSHPrivateKey(privateKey string) (ssh.Signer, error) {
	key, err := ssh.ParseRawPrivateKey([]byte(privateKey))
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(key)
}
