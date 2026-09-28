package router

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"rokishi-back/internal/http/authctx"
	"rokishi-back/internal/http/handlers"
	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

func authenticationMiddleware(security handlers.SecurityService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if security == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := security.Authenticate(r.Context(), handlers.SessionToken(r))
			if err != nil {
				if errors.Is(err, service.ErrUnauthorized) {
					handlers.WriteError(w, http.StatusUnauthorized, "authentication_required", "Debes iniciar sesion")
					return
				}
				handlers.WriteError(w, http.StatusInternalServerError, "internal_error", "No se pudo validar la sesion")
				return
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithUser(r.Context(), user)))
		})
	}
}

func requireAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := authctx.User(r.Context())
		if !ok || user.Role != models.RoleAdmin {
			handlers.WriteError(w, http.StatusForbidden, "forbidden", "Se requiere el rol de administrador")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func auditMiddleware(security handlers.SecurityService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if security == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r)
			if recorder.status >= http.StatusBadRequest {
				return
			}
			user, ok := authctx.User(r.Context())
			if !ok {
				return
			}
			resource := chi.RouteContext(r.Context()).RoutePattern()
			if resource == "" {
				resource = r.URL.Path
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = security.RecordAudit(ctx, service.AuditInput{
				UserID: user.ID, UserName: user.Name, Action: r.Method, Resource: resource,
				HTTPStatus: recorder.status, IPAddress: handlers.ClientIP(r), UserAgent: r.UserAgent(),
			})
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}
