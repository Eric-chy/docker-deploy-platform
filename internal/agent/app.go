package agent

import (
	"net/http"
	"sync"

	"deploy-platform/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
)

type App struct {
	config Config
	tasks  sync.Map
}

func New(config Config) *App {
	return &App{config: config}
}

func (app *App) Routes() http.Handler {
	router := chi.NewRouter()
	router.Use(app.auth)
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
	})
	router.Post("/api/deploy", app.deploy)
	router.Post("/api/rollback", app.notImplemented)
	return router
}

func (app *App) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+app.config.APIKey {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) notImplemented(w http.ResponseWriter, _ *http.Request) {
	httpx.Error(w, http.StatusNotImplemented, "deployment execution is not implemented")
}
