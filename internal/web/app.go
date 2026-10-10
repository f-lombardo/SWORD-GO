package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sword-go/internal/app/backups"
	"sword-go/internal/app/integrations"
	"sword-go/internal/app/servers"
	"sword-go/internal/app/sites"
	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"

	_ "modernc.org/sqlite"
)

type App struct {
	server              *http.Server
	db                  *sql.DB
	templates           *template.Template
	serversService      *servers.Service
	serversStore        *servers.Store
	sitesService        *sites.Service
	sitesStore          *sites.Store
	backupsService      *backups.Service
	backupsStore        *backups.Store
	integrationsService *integrations.Service
	integrationsStore   *integrations.Store
	auth                *authService
	loginLimiter        *loginRateLimiter
	callbackGuard       *callbackReplayGuard
}

func (a *App) String() string {
	return a.server.Addr
}

func NewApp() (*App, error) {
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	dsn := cmpOr(os.Getenv("SWORD_GO_DB_DSN"), "file:sword-go.db?cache=shared&mode=rwc")
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	store := servers.NewStore(database)
	sitesStore := sites.NewStore(database)
	backupsStore := backups.NewStore(database)
	integrationsStore := integrations.NewStore(database)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = store.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	if err = sitesStore.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	if err = backupsStore.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	if err = integrationsStore.EnsureSchema(ctx); err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	service := servers.NewService(
		store,
		digitalocean.NewCreator(httpClient),
		hetzner.NewCreator(httpClient),
	)
	baseURL := cmpOr(os.Getenv("SWORD_GO_BASE_URL"), "http://localhost:"+cmpOr(os.Getenv("SWORD_GO_HTTP_PORT"), "8088"))
	sitesService := sites.NewService(sitesStore, store, baseURL)
	backupsService := backups.NewService(backupsStore, store, sitesStore)
	integrationsService := integrations.NewService(integrationsStore)
	secureCookies := parseBoolOrDefault(os.Getenv("SWORD_GO_SECURE_COOKIES"), false)
	strictAuth := parseBoolOrDefault(os.Getenv("SWORD_GO_AUTH_STRICT"), secureCookies)
	loginMaxAttempts := parseIntOrDefault(os.Getenv("SWORD_GO_LOGIN_MAX_ATTEMPTS"), 5)
	loginWindowSeconds := parseIntOrDefault(os.Getenv("SWORD_GO_LOGIN_WINDOW_SECONDS"), 600)
	auth, err := newAuthService(
		cmpOr(os.Getenv("SWORD_GO_ADMIN_EMAIL"), "admin@example.com"),
		cmpOr(os.Getenv("SWORD_GO_ADMIN_PASSWORD"), "password"),
		strings.TrimSpace(os.Getenv("SWORD_GO_ADMIN_PASSWORD_HASH")),
		cmpOr(os.Getenv("SWORD_GO_SESSION_SECRET"), "change-me-in-env"),
		secureCookies,
		strictAuth,
	)
	if err != nil {
		return nil, err
	}

	app := &App{
		db:                  database,
		templates:           templates,
		serversService:      service,
		serversStore:        store,
		sitesService:        sitesService,
		sitesStore:          sitesStore,
		backupsService:      backupsService,
		backupsStore:        backupsStore,
		integrationsService: integrationsService,
		integrationsStore:   integrationsStore,
		loginLimiter:        newLoginRateLimiter(loginMaxAttempts, time.Duration(loginWindowSeconds)*time.Second),
		callbackGuard:       newCallbackReplayGuard(10 * time.Minute),
		auth:                auth,
	}

	port := cmpOr(os.Getenv("SWORD_GO_HTTP_PORT"), "8088")
	address := ":" + port
	app.server = &http.Server{
		Addr:              address,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	go a.startBackupDispatcher(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()

	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (a *App) startBackupDispatcher(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case tickTime := <-ticker.C:
			_, _ = a.backupsService.DispatchDueBackups(context.Background(), tickTime.UTC())
		}
	}
}

func (a *App) renderTemplate(w http.ResponseWriter, r *http.Request, name string, data any) {
	if asMap, ok := data.(map[string]any); ok {
		if _, exists := asMap["CSRFToken"]; !exists {
			asMap["CSRFToken"] = a.auth.ensureCSRFCookie(w, r)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parsePositiveID(pathValue string) (int64, error) {
	value, err := strconv.ParseInt(pathValue, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid id")
	}
	return value, nil
}

func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseBoolOrDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseIntOrDefault(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
