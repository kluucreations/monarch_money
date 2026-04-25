package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPkceVerify(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	if !pkceVerify(verifier, challenge) {
		t.Error("expected valid verifier/challenge pair to pass")
	}
	if pkceVerify("wrongverifier", challenge) {
		t.Error("expected wrong verifier to fail")
	}
	if pkceVerify(verifier, "wrongchallenge") {
		t.Error("expected wrong challenge to fail")
	}
}

func TestCodeStore(t *testing.T) {
	store := &codeStore{codes: make(map[string]authCode)}
	ac := authCode{challenge: "abc", redirectURI: "https://example.com", expiresAt: time.Now().Add(time.Minute)}

	store.save("code1", ac)

	got, ok := store.take("code1")
	if !ok {
		t.Fatal("expected to find saved code")
	}
	if got.challenge != "abc" {
		t.Errorf("got challenge %q, want %q", got.challenge, "abc")
	}

	// code should be consumed
	_, ok = store.take("code1")
	if ok {
		t.Error("expected code to be consumed after take")
	}

	// missing code
	_, ok = store.take("nonexistent")
	if ok {
		t.Error("expected missing code to return false")
	}
}

const testClientSecret = "test-client-secret"

func newOAuthMux(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	mux := http.NewServeMux()
	registerOAuthHandlers(mux, "test-monarch-token", testClientSecret, "https://example.com")
	return mux, "test-monarch-token"
}

func TestWellKnownMetadata(t *testing.T) {
	mux, _ := newOAuthMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["token_endpoint"] != "https://example.com/token" {
		t.Errorf("unexpected token_endpoint: %v", body["token_endpoint"])
	}
	if body["authorization_endpoint"] != "https://example.com/authorize" {
		t.Errorf("unexpected authorization_endpoint: %v", body["authorization_endpoint"])
	}
}

func TestAuthorizeRejectsUnknownClient(t *testing.T) {
	mux, _ := newOAuthMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/authorize?client_id=unknown&redirect_uri=https://cb.example.com&code_challenge=abc&code_challenge_method=S256", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for unknown client_id", rec.Code)
	}
}

func TestAuthorizeRejectsMissingPKCE(t *testing.T) {
	mux, _ := newOAuthMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/authorize?client_id=monarch-mcp&redirect_uri=https://cb.example.com", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for missing PKCE", rec.Code)
	}
}

func TestAuthorizeRejectsWrongMethod(t *testing.T) {
	mux, _ := newOAuthMux(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/authorize?client_id=monarch-mcp&redirect_uri=https://cb.example.com&code_challenge=abc&code_challenge_method=plain", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for non-S256 method", rec.Code)
	}
}

func pkceChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func tokenRequest(form url.Values) *http.Request {
	req := httptest.NewRequest("POST", "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestFullOAuthPKCEFlow(t *testing.T) {
	mux, wantToken := newOAuthMux(t)

	verifier := "test-verifier-long-enough-for-pkce-spec-minimum"
	challenge := pkceChallenge(verifier)
	redirectURI := "https://claude.ai/callback"

	// Step 1: authorize
	authorizeURL := "/authorize?client_id=monarch-mcp&redirect_uri=" + url.QueryEscape(redirectURI) +
		"&code_challenge=" + challenge + "&code_challenge_method=S256&state=mystate"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", authorizeURL, nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("authorize: got status %d, want 302", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, redirectURI) {
		t.Fatalf("redirect location %q does not start with redirect_uri", location)
	}
	if !strings.Contains(location, "state=mystate") {
		t.Error("state not preserved in redirect")
	}

	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect location: %v", err)
	}
	code := parsed.Query().Get("code")
	if code == "" {
		t.Fatal("no code in redirect")
	}

	// Step 2: exchange code for token
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
		"client_secret": {testClientSecret},
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(form))

	if rec.Code != http.StatusOK {
		t.Fatalf("token: got status %d body: %s", rec.Code, rec.Body.String())
	}
	var tokenResp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&tokenResp); err != nil {
		t.Fatalf("failed to decode token response: %v", err)
	}
	if tokenResp["access_token"] != wantToken {
		t.Errorf("got access_token %q, want %q", tokenResp["access_token"], wantToken)
	}
	if tokenResp["token_type"] != "Bearer" {
		t.Errorf("got token_type %q, want Bearer", tokenResp["token_type"])
	}
}

func TestTokenRejectsWrongVerifier(t *testing.T) {
	mux, _ := newOAuthMux(t)

	verifier := "correct-verifier-long-enough-for-pkce"
	challenge := pkceChallenge(verifier)
	redirectURI := "https://claude.ai/callback"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET",
		"/authorize?client_id=monarch-mcp&redirect_uri="+url.QueryEscape(redirectURI)+
			"&code_challenge="+challenge+"&code_challenge_method=S256", nil))

	parsed, _ := url.Parse(rec.Header().Get("Location"))
	code := parsed.Query().Get("code")

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {"wrong-verifier"},
		"client_secret": {testClientSecret},
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(form))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for wrong verifier", rec.Code)
	}
}

func TestTokenRejectsUsedCode(t *testing.T) {
	mux, _ := newOAuthMux(t)

	verifier := "test-verifier-long-enough-for-pkce-spec"
	challenge := pkceChallenge(verifier)
	redirectURI := "https://claude.ai/callback"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET",
		"/authorize?client_id=monarch-mcp&redirect_uri="+url.QueryEscape(redirectURI)+
			"&code_challenge="+challenge+"&code_challenge_method=S256", nil))

	parsed, _ := url.Parse(rec.Header().Get("Location"))
	code := parsed.Query().Get("code")

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"client_secret": {testClientSecret},
	}

	// First use: should succeed
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(form))
	if rec.Code != http.StatusOK {
		t.Fatalf("first token exchange failed: %d", rec.Code)
	}

	// Second use: should fail
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(form))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400 for reused code", rec.Code)
	}
}

func TestTokenRejectsWrongClientSecret(t *testing.T) {
	mux, _ := newOAuthMux(t)

	verifier := "test-verifier-long-enough-for-pkce-spec"
	challenge := pkceChallenge(verifier)
	redirectURI := "https://claude.ai/callback"

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET",
		"/authorize?client_id=monarch-mcp&redirect_uri="+url.QueryEscape(redirectURI)+
			"&code_challenge="+challenge+"&code_challenge_method=S256", nil))

	parsed, _ := url.Parse(rec.Header().Get("Location"))
	code := parsed.Query().Get("code")

	for _, secret := range []string{"", "wrong-secret"} {
		form := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"code_verifier": {verifier},
			"client_secret": {secret},
		}
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, tokenRequest(form))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("client_secret=%q: got %d, want 401", secret, rec.Code)
		}
	}
}

func TestTokenOptionsPreflight(t *testing.T) {
	mux, _ := newOAuthMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/token", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204 for OPTIONS preflight", rec.Code)
	}
}
