package router

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"rokishi-back/internal/http/handlers"
)

func New(ping func(context.Context) error) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", handlers.Health(ping))
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusNotFound, "not_found", "Ruta no encontrada")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		handlers.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Metodo no permitido")
	})
	return r
}
