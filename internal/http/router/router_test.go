package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ping       func(context.Context) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "postgres available",
			ping:       func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok","database":"up"}`,
		},
		{
			name:       "postgres unavailable",
			ping:       func(context.Context) error { return errors.New("connection refused") },
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `{"status":"error","database":"down","error":{"code":"database_unavailable","message":"PostgreSQL no disponible"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			response := httptest.NewRecorder()
			New(tc.ping).ServeHTTP(response, request)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tc.wantStatus)
			}
			if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Fatalf("content type = %q", got)
			}
			var got, want any
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.wantBody), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("body = %s, want %s", response.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestJSONErrors(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
		status int
		code   string
	}{
		{http.MethodGet, "/missing", http.StatusNotFound, "not_found"},
		{http.MethodPost, "/api/health", http.StatusMethodNotAllowed, "method_not_allowed"},
	} {
		response := httptest.NewRecorder()
		New(func(context.Context) error { return nil }).ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != tc.status {
			t.Fatalf("status = %d, want %d", response.Code, tc.status)
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != tc.code {
			t.Fatalf("error code = %q, want %q", body.Error.Code, tc.code)
		}
	}
}
