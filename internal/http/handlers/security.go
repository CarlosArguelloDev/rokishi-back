package handlers

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"rokishi-back/internal/http/authctx"
	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

const SessionCookieName = "rokishi_session"

type SecurityService interface {
	SetupRequired(context.Context) (bool, error)
	Bootstrap(context.Context, string, service.CreateUserInput) (models.User, string, time.Time, error)
	Login(context.Context, string, string) (models.User, string, time.Time, error)
	Authenticate(context.Context, string) (models.User, error)
	Logout(context.Context, string) error
	ListUsers(context.Context) ([]models.User, error)
	CreateUser(context.Context, service.CreateUserInput) (models.User, error)
	UpdateUser(context.Context, int64, int64, service.UpdateUserInput) (models.User, error)
	RecordAudit(context.Context, service.AuditInput) error
	ListAudit(context.Context, int) ([]models.AuditEntry, error)
}

type SecurityHandler struct {
	service SecurityService
	limiter *loginLimiter
}

func NewSecurityHandler(securityService SecurityService) *SecurityHandler {
	return &SecurityHandler{service: securityService, limiter: newLoginLimiter(5, 10*time.Minute)}
}

type authStatus struct {
	SetupRequired bool         `json:"configuracion_requerida"`
	User          *models.User `json:"usuario"`
}

type bootstrapRequest struct {
	SetupCode string `json:"codigo_configuracion"`
	Name      string `json:"nombre"`
	Email     string `json:"correo"`
	Password  string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"correo"`
	Password string `json:"password"`
}

type createUserRequest struct {
	Name     string `json:"nombre"`
	Email    string `json:"correo"`
	Password string `json:"password"`
	Role     string `json:"rol"`
}

type updateUserRequest struct {
	Name     optional[string] `json:"nombre"`
	Email    optional[string] `json:"correo"`
	Password optional[string] `json:"password"`
	Role     optional[string] `json:"rol"`
	Active   optional[bool]   `json:"activo"`
}

func (h *SecurityHandler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	required, err := h.service.SetupRequired(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "No se pudo consultar la configuracion de acceso")
		return
	}
	var user *models.User
	if token := SessionToken(r); token != "" {
		authenticated, err := h.service.Authenticate(r.Context(), token)
		if err == nil {
			user = &authenticated
		} else if !errors.Is(err, service.ErrUnauthorized) {
			WriteError(w, http.StatusInternalServerError, "internal_error", "No se pudo validar la sesion")
			return
		}
	}
	writeJSON(w, http.StatusOK, dataResponse[authStatus]{Data: authStatus{SetupRequired: required, User: user}})
}

func (h *SecurityHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	key := "setup|" + ClientIP(r)
	if !h.limiter.Allow(key) {
		WriteError(w, http.StatusTooManyRequests, "rate_limited", "Demasiados intentos. Espera unos minutos e intenta de nuevo")
		return
	}
	var request bootstrapRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	user, token, expiresAt, err := h.service.Bootstrap(r.Context(), request.SetupCode, service.CreateUserInput{
		Name: request.Name, Email: request.Email, Password: request.Password, Role: models.RoleAdmin,
	})
	if err != nil {
		h.limiter.Failure(key)
		writeSecurityError(w, err)
		return
	}
	h.limiter.Reset(key)
	setSessionCookie(w, r, token, expiresAt)
	h.recordAccess(r, user, "SETUP", http.StatusCreated)
	writeJSON(w, http.StatusCreated, dataResponse[models.User]{Data: user})
}

func (h *SecurityHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request loginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	ipKey := "ip|" + ClientIP(r)
	accountKey := "account|" + strings.ToLower(strings.TrimSpace(request.Email))
	if !h.limiter.Allow(ipKey) || !h.limiter.Allow(accountKey) {
		WriteError(w, http.StatusTooManyRequests, "rate_limited", "Demasiados intentos. Espera unos minutos e intenta de nuevo")
		return
	}
	user, token, expiresAt, err := h.service.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			h.limiter.Failure(ipKey)
			h.limiter.Failure(accountKey)
		}
		writeSecurityError(w, err)
		return
	}
	h.limiter.Reset(ipKey)
	h.limiter.Reset(accountKey)
	setSessionCookie(w, r, token, expiresAt)
	h.recordAccess(r, user, "LOGIN", http.StatusOK)
	writeJSON(w, http.StatusOK, dataResponse[models.User]{Data: user})
}

func (h *SecurityHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := h.service.Logout(r.Context(), SessionToken(r)); err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "No se pudo cerrar la sesion")
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, dataResponse[map[string]bool]{Data: map[string]bool{"sesion_cerrada": true}})
}

func (h *SecurityHandler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, _ := authctx.User(r.Context())
	writeJSON(w, http.StatusOK, dataResponse[models.User]{Data: user})
}

func (h *SecurityHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal_error", "No se pudieron consultar los usuarios")
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.User]{Data: users})
}

func (h *SecurityHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	user, err := h.service.CreateUser(r.Context(), service.CreateUserInput{
		Name: request.Name, Email: request.Email, Password: request.Password, Role: request.Role,
	})
	if err != nil {
		writeSecurityError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataResponse[models.User]{Data: user})
}

func (h *SecurityHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	actor, _ := authctx.User(r.Context())
	var request updateUserRequest
	if err := decodeJSON(w, r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_json", "El cuerpo JSON no es valido")
		return
	}
	user, err := h.service.UpdateUser(r.Context(), actor.ID, id, service.UpdateUserInput{
		Name: toField(request.Name), Email: toField(request.Email), Password: toField(request.Password),
		Role: toField(request.Role), Active: toField(request.Active),
	})
	if err != nil {
		writeSecurityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[models.User]{Data: user})
}

func (h *SecurityHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	limit, err := service.ParseAuditLimit(r.URL.Query().Get("limite"))
	if err != nil {
		writeSecurityError(w, err)
		return
	}
	entries, err := h.service.ListAudit(r.Context(), limit)
	if err != nil {
		writeSecurityError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataResponse[[]models.AuditEntry]{Data: entries})
}

func (h *SecurityHandler) recordAccess(r *http.Request, user models.User, action string, status int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.service.RecordAudit(ctx, service.AuditInput{
		UserID: user.ID, UserName: user.Name, Action: action, Resource: r.URL.Path,
		HTTPStatus: status, IPAddress: ClientIP(r), UserAgent: r.UserAgent(),
	})
}

func SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Value: token, Path: "/api", HttpOnly: true,
		Secure: requestIsSecure(r), SameSite: sameSite(r), Expires: expiresAt,
		MaxAge: int(time.Until(expiresAt).Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Value: "", Path: "/api", HttpOnly: true,
		Secure: requestIsSecure(r), SameSite: sameSite(r), MaxAge: -1, Expires: time.Unix(1, 0),
	})
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func sameSite(r *http.Request) http.SameSite {
	if requestIsSecure(r) {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func ClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeSecurityError(w http.ResponseWriter, err error) {
	var validationError *service.ValidationError
	switch {
	case errors.As(err, &validationError):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed", validationError.Message)
	case errors.Is(err, service.ErrUnauthorized), errors.Is(err, service.ErrInvalidSetupCode):
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Las credenciales no son validas")
	case errors.Is(err, service.ErrForbidden):
		WriteError(w, http.StatusForbidden, "forbidden", "No tienes permisos para realizar esta operacion")
	case errors.Is(err, service.ErrSetupComplete):
		WriteError(w, http.StatusConflict, "setup_complete", "La configuracion inicial ya fue completada")
	case errors.Is(err, service.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "Usuario no encontrado")
	case errors.Is(err, service.ErrConflict):
		WriteError(w, http.StatusConflict, "email_in_use", "Ya existe un usuario con ese correo")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
	}
}

type loginAttempt struct {
	failures int
	resetAt  time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	maximum  int
	window   time.Duration
}

func newLoginLimiter(maximum int, window time.Duration) *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt), maximum: maximum, window: window}
}

func (l *loginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.attempts) > 1000 {
		for attemptKey, attempt := range l.attempts {
			if now.After(attempt.resetAt) {
				delete(l.attempts, attemptKey)
			}
		}
	}
	attempt, exists := l.attempts[key]
	if exists && now.After(attempt.resetAt) {
		delete(l.attempts, key)
		return true
	}
	return !exists || attempt.failures < l.maximum
}

func (l *loginLimiter) Failure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt, exists := l.attempts[key]
	if !exists || time.Now().After(attempt.resetAt) {
		attempt = loginAttempt{resetAt: time.Now().Add(l.window)}
	}
	attempt.failures++
	l.attempts[key] = attempt
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}
