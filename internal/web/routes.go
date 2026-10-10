package web

import "net/http"

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", a.rootHandler)
	mux.HandleFunc("/healthz", a.healthzHandler)

	mux.Handle("/login", a.requireCSRF(http.HandlerFunc(a.loginHandler)))
	mux.Handle("/logout", a.requireAuth(a.requireCSRF(http.HandlerFunc(a.logoutHandler))))

	mux.Handle("/public/servers/", http.HandlerFunc(a.publicServersHandler))
	mux.Handle("/public/sites/", http.HandlerFunc(a.publicSitesHandler))

	protected := func(handler http.HandlerFunc) http.Handler {
		return a.requireAuth(a.requireCSRF(handler))
	}

	mux.Handle("/servers", protected(a.serversIndexCreateHandler))
	mux.Handle("/servers/", protected(a.serversDetailHandler))
	mux.Handle("/servers/generate-name", protected(a.generateNameHandler))
	mux.Handle("/sites", protected(a.sitesIndexCreateHandler))
	mux.Handle("/sites/", protected(a.sitesDetailHandler))
	mux.Handle("/backup-destinations", protected(a.backupDestinationsIndexCreateHandler))
	mux.Handle("/backup-destinations/", protected(a.backupDestinationsDetailHandler))
	mux.Handle("/backup-schedules", protected(a.backupSchedulesIndexHandler))
	mux.Handle("/settings/integrations", protected(a.integrationsIndexCreateHandler))
	mux.Handle("/settings/integrations/", protected(a.integrationsDetailHandler))
	mux.Handle("/cloudflare", protected(a.cloudflareIndexHandler))
	mux.Handle("/cloudflare/", protected(a.cloudflareDetailHandler))

	return mux
}
