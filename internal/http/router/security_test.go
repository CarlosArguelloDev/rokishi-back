package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rokishi-back/internal/models"
	"rokishi-back/internal/service"
)

type fakeSecurityService struct {
	user models.User
}

func (f fakeSecurityService) SetupRequired(context.Context) (bool, error) { return false, nil }
func (f fakeSecurityService) Bootstrap(context.Context, string, service.CreateUserInput) (models.User, string, time.Time, error) {
	return f.user, "session-token", time.Now().Add(time.Hour), nil
}
func (f fakeSecurityService) Login(_ context.Context, email, password string) (models.User, string, time.Time, error) {
	if email == "admin@example.com" && password == "valid-password" {
		return f.user, "session-token", time.Now().Add(time.Hour), nil
	}
	return models.User{}, "", time.Time{}, service.ErrUnauthorized
}
func (f fakeSecurityService) Authenticate(_ context.Context, token string) (models.User, error) {
	if token != "session-token" {
		return models.User{}, service.ErrUnauthorized
	}
	return f.user, nil
}
func (fakeSecurityService) Logout(context.Context, string) error { return nil }
func (f fakeSecurityService) ListUsers(context.Context) ([]models.User, error) {
	return []models.User{f.user}, nil
}
func (f fakeSecurityService) CreateUser(context.Context, service.CreateUserInput) (models.User, error) {
	return f.user, nil
}
func (f fakeSecurityService) UpdateUser(context.Context, int64, int64, service.UpdateUserInput) (models.User, error) {
	return f.user, nil
}
func (fakeSecurityService) RecordAudit(context.Context, service.AuditInput) error { return nil }
func (fakeSecurityService) ListAudit(context.Context, int) ([]models.AuditEntry, error) {
	return nil, nil
}

func TestProtectedRoutesRequireSession(t *testing.T) {
	security := fakeSecurityService{user: models.User{ID: 1, Name: "Admin", Role: models.RoleAdmin, Active: true}}
	handler := New(Dependencies{Security: security})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: "rokishi_session", Value: "session-token"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
}

func TestAdminRoutesRejectOperator(t *testing.T) {
	security := fakeSecurityService{user: models.User{ID: 2, Name: "Operador", Role: models.RoleOperator, Active: true}}
	request := httptest.NewRequest(http.MethodGet, "/api/usuarios", nil)
	request.AddCookie(&http.Cookie{Name: "rokishi_session", Value: "session-token"})
	response := httptest.NewRecorder()
	New(Dependencies{Security: security}).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestLoginUsesSecureCrossSiteCookie(t *testing.T) {
	security := fakeSecurityService{user: models.User{ID: 1, Name: "Admin", Role: models.RoleAdmin, Active: true}}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"correo":"admin@example.com","password":"valid-password"}`))
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	New(Dependencies{Security: security}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	cookie := response.Header().Get("Set-Cookie")
	for _, attribute := range []string{"HttpOnly", "Secure", "SameSite=None"} {
		if !strings.Contains(cookie, attribute) {
			t.Fatalf("cookie %q does not contain %q", cookie, attribute)
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	security := fakeSecurityService{}
	handler := New(Dependencies{Security: security})
	for attempt := 1; attempt <= 6; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"correo":"user@example.com","password":"wrong-password"}`))
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("attempt %d: status = %d, want %d", attempt, response.Code, want)
		}
	}
}
