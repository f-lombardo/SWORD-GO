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
	"time"

	"sword-go/internal/app/servers"
	"sword-go/internal/cloud/digitalocean"
	"sword-go/internal/cloud/hetzner"

	_ "modernc.org/sqlite"
)

type App struct {
	server         *http.Server
	templates      *template.Template
	serversService *servers.Service
	serversStore   *servers.Store
	auth           *authService
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = store.EnsureSchema(ctx); err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	service := servers.NewService(
		store,
		digitalocean.NewCreator(httpClient),
		hetzner.NewCreator(httpClient),
	)

	app := &App{
		templates:      templates,
		serversService: service,
		serversStore:   store,
		auth: newAuthService(
			cmpOr(os.Getenv("SWORD_GO_ADMIN_EMAIL"), "admin@example.com"),
			cmpOr(os.Getenv("SWORD_GO_ADMIN_PASSWORD"), "password"),
			cmpOr(os.Getenv("SWORD_GO_SESSION_SECRET"), "change-me-in-env"),
		),
	}

	port := cmpOr(os.Getenv("SWORD_GO_HTTP_PORT"), "8088")
	address := ":" + port
	app.server = &http.Server{
		Addr:              address,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return app, nil
}

func (a *App) Run(ctx context.Context) error {
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

func (a *App) renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *App) parseServerID(pathValue string) (int64, error) {
	value, err := strconv.ParseInt(pathValue, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid server id")
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
