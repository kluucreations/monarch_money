package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const oauthClientID = "monarch-mcp"

type authCode struct {
	challenge   string
	redirectURI string
	expiresAt   time.Time
}

type codeStore struct {
	mu    sync.Mutex
	codes map[string]authCode
}

func (cs *codeStore) save(code string, ac authCode) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.codes[code] = ac
}

func (cs *codeStore) take(code string) (authCode, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	ac, ok := cs.codes[code]
	if !ok {
		return authCode{}, false
	}
	delete(cs.codes, code)
	return ac, true
}

func pkceVerify(verifier, challenge string) bool {
	h := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == challenge
}

func registerOAuthHandlers(mux *http.ServeMux, monarchToken, clientSecret, baseURL string) {
	store := &codeStore{codes: make(map[string]authCode)}

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 baseURL,
			"authorization_endpoint": baseURL + "/authorize",
			"token_endpoint":         baseURL + "/token",
			"response_types_supported": []string{"code"},
			"grant_types_supported":    []string{"authorization_code"},
			"code_challenge_methods_supported": []string{"S256"},
		})
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		clientID := q.Get("client_id")
		redirectURI := q.Get("redirect_uri")
		challenge := q.Get("code_challenge")
		method := q.Get("code_challenge_method")
		state := q.Get("state")

		if clientID != oauthClientID {
			http.Error(w, "unknown client_id", http.StatusBadRequest)
			return
		}
		if challenge == "" || method != "S256" {
			http.Error(w, "PKCE S256 required", http.StatusBadRequest)
			return
		}
		if redirectURI == "" {
			http.Error(w, "redirect_uri required", http.StatusBadRequest)
			return
		}

		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		code := base64.RawURLEncoding.EncodeToString(b)

		store.save(code, authCode{
			challenge:   challenge,
			redirectURI: redirectURI,
			expiresAt:   time.Now().Add(5 * time.Minute),
		})

		log.Printf("[oauth] issued code for client=%s redirect=%s", clientID, redirectURI)

		redirect := fmt.Sprintf("%s?code=%s", redirectURI, code)
		if state != "" {
			redirect += "&state=" + state
		}
		http.Redirect(w, r, redirect, http.StatusFound)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		if r.FormValue("client_secret") != clientSecret {
			http.Error(w, "invalid client_secret", http.StatusUnauthorized)
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "authorization_code" {
			http.Error(w, "unsupported grant_type", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		verifier := r.FormValue("code_verifier")

		ac, ok := store.take(code)
		if !ok {
			http.Error(w, "invalid or expired code", http.StatusBadRequest)
			return
		}
		if time.Now().After(ac.expiresAt) {
			http.Error(w, "code expired", http.StatusBadRequest)
			return
		}

		redirectURI := r.FormValue("redirect_uri")
		if redirectURI != "" && !strings.HasPrefix(ac.redirectURI, strings.Split(redirectURI, "?")[0]) {
			http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
			return
		}

		if !pkceVerify(verifier, ac.challenge) {
			http.Error(w, "code_verifier mismatch", http.StatusBadRequest)
			return
		}

		log.Printf("[oauth] token issued")

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": monarchToken,
			"token_type":   "Bearer",
		})
	})
}
