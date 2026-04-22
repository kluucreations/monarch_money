package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestWithAuthMissingHeader(t *testing.T) {
	h := withAuth(http.HandlerFunc(okHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 for missing Authorization", rec.Code)
	}
}

func TestWithAuthInvalidFormat(t *testing.T) {
	h := withAuth(http.HandlerFunc(okHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic abc123")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 for non-Bearer auth", rec.Code)
	}
}

func TestWithAuthBearerOnly(t *testing.T) {
	h := withAuth(http.HandlerFunc(okHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401 for empty Bearer token", rec.Code)
	}
}

func TestWithAuthValidToken(t *testing.T) {
	var gotToken string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken, _ = r.Context().Value(monarchTokenKey).(string)
		w.WriteHeader(http.StatusOK)
	})

	h := withAuth(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer mytoken1234")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200 for valid Bearer token", rec.Code)
	}
	if gotToken != "mytoken1234" {
		t.Errorf("got token %q in context, want %q", gotToken, "mytoken1234")
	}
}

func TestWithCORSHeaders(t *testing.T) {
	h := withCORS(http.HandlerFunc(okHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin: *")
	}
	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
}

func TestWithCORSPreflight(t *testing.T) {
	h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called for OPTIONS")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204 for OPTIONS preflight", rec.Code)
	}
}
