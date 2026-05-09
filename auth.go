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
		log.Printf("[oauth] metadata requested from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                           baseURL,
			"authorization_endpoint":           baseURL + "/authorize",
			"token_endpoint":                   baseURL + "/token",
			"response_types_supported":         []string{"code"},
			"grant_types_supported":            []string{"authorization_code"},
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

		log.Printf("[oauth/authorize] client_id=%s redirect_uri=%s method=%s state=%s", clientID, redirectURI, method, state)

		if clientID != oauthClientID {
			log.Printf("[oauth/authorize] rejected: unknown client_id=%s", clientID)
			http.Error(w, "unknown client_id", http.StatusBadRequest)
			return
		}
		if challenge == "" || method != "S256" {
			log.Printf("[oauth/authorize] rejected: bad PKCE params challenge=%q method=%q", challenge, method)
			http.Error(w, "PKCE S256 required", http.StatusBadRequest)
			return
		}
		if redirectURI == "" {
			log.Printf("[oauth/authorize] rejected: missing redirect_uri")
			http.Error(w, "redirect_uri required", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(redirectURI, "https://claude.ai/") {
			log.Printf("[oauth/authorize] rejected: redirect_uri not allowed: %s", redirectURI)
			http.Error(w, "redirect_uri not allowed", http.StatusBadRequest)
			return
		}

		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			log.Printf("[oauth/authorize] failed to generate code: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		code := base64.RawURLEncoding.EncodeToString(b)

		store.save(code, authCode{
			challenge:   challenge,
			redirectURI: redirectURI,
			expiresAt:   time.Now().Add(5 * time.Minute),
		})

		log.Printf("[oauth/authorize] issued code=%s... for client=%s redirect=%s", code[:6], clientID, redirectURI)

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
			log.Printf("[oauth/token] failed to parse form: %v", err)
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}

		log.Printf("[oauth/token] grant_type=%s code=%s verifier_len=%d auth_header=%s",
			r.FormValue("grant_type"),
			obfuscate(r.FormValue("code")),
			len(r.FormValue("code_verifier")),
			obfuscate(r.Header.Get("Authorization")),
		)

		// Log what client secret Claude is sending (for debugging).
		secret := r.FormValue("client_secret")
		if secret == "" {
			_, secret, _ = r.BasicAuth()
		}
		log.Printf("[oauth/token] client_secret from client=%s server=%s", secret, obfuscate(clientSecret))

		grantType := r.FormValue("grant_type")
		if grantType != "authorization_code" {
			log.Printf("[oauth/token] rejected: unsupported grant_type=%s", grantType)
			http.Error(w, "unsupported grant_type", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		verifier := r.FormValue("code_verifier")

		ac, ok := store.take(code)
		if !ok {
			log.Printf("[oauth/token] rejected: code not found or already used")
			http.Error(w, "invalid or expired code", http.StatusBadRequest)
			return
		}
		if time.Now().After(ac.expiresAt) {
			log.Printf("[oauth/token] rejected: code expired")
			http.Error(w, "code expired", http.StatusBadRequest)
			return
		}

		redirectURI := r.FormValue("redirect_uri")
		if redirectURI != "" && !strings.HasPrefix(ac.redirectURI, strings.Split(redirectURI, "?")[0]) {
			log.Printf("[oauth/token] rejected: redirect_uri mismatch got=%s stored=%s", redirectURI, ac.redirectURI)
			http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
			return
		}

		if !pkceVerify(verifier, ac.challenge) {
			log.Printf("[oauth/token] rejected: code_verifier mismatch")
			http.Error(w, "code_verifier mismatch", http.StatusBadRequest)
			return
		}

		log.Printf("[oauth/token] success — issuing access token")

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": monarchToken,
			"token_type":   "Bearer",
		})
	})
}
