package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status   string       `json:"status"`
	Database string       `json:"database"`
	Error    *errorDetail `json:"error,omitempty"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	response := errorResponse{Error: errorDetail{Code: code, Message: message}}
	_ = json.NewEncoder(w).Encode(response)
}

func Health(ping func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(healthResponse{
				Status:   "error",
				Database: "down",
				Error:    &errorDetail{Code: "database_unavailable", Message: "PostgreSQL no disponible"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok", Database: "up"})
	}
}
