package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ── Environment config ──────────────────────────────────

type Config struct {
	Port                   string
	OAuthAuthorizePassword string
	MCPBearerToken         string
	OAuthServerBaseURL     string
	MCPResourceURL         string
}

func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	baseURL := os.Getenv("OAUTH_SERVER_BASE_URL")
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	resourceURL := os.Getenv("MCP_RESOURCE_URL")
	if resourceURL == "" {
		resourceURL = baseURL
	}
	return Config{
		Port:                   port,
		OAuthAuthorizePassword: os.Getenv("OAUTH_AUTHORIZE_PASSWORD"),
		MCPBearerToken:         os.Getenv("MCP_BEARER_TOKEN"),
		OAuthServerBaseURL:     baseURL,
		MCPResourceURL:         resourceURL,
	}
}

// ── Data types ──────────────────────────────────────────

type Client struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type AuthCode struct {
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	Resource      string
	CreatedAt     time.Time
}

type RefreshToken struct {
	ClientID  string
	CreatedAt time.Time
}

// ── In-memory stores ────────────────────────────────────

type Store struct {
	mu            sync.RWMutex
	clients       map[string]*Client       // client_id → Client
	authCodes     map[string]*AuthCode     // code → AuthCode
	refreshTokens map[string]*RefreshToken // token → RefreshToken
}

func NewStore() *Store {
	s := &Store{
		clients:       make(map[string]*Client),
		authCodes:     make(map[string]*AuthCode),
		refreshTokens: make(map[string]*RefreshToken),
	}
	// Start cleanup goroutine for expired auth codes
	go s.cleanupLoop()
	return s
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for code, ac := range s.authCodes {
			if now.Sub(ac.CreatedAt) > 5*time.Minute {
				delete(s.authCodes, code)
			}
		}
		// Clean refresh tokens older than 30 days
		for token, rt := range s.refreshTokens {
			if now.Sub(rt.CreatedAt) > 30*24*time.Hour {
				delete(s.refreshTokens, token)
			}
		}
		s.mu.Unlock()
	}
}

// ── Helpers ─────────────────────────────────────────────

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeOAuthError(w http.ResponseWriter, status int, errCode, description string) {
	writeJSON(w, status, map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}

// ── PKCE verification ───────────────────────────────────

func verifyPKCE(codeVerifier, codeChallenge string) bool {
	h := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(h[:])
	return computed == codeChallenge
}

// ── Login page template ─────────────────────────────────

var loginTemplate = template.Must(template.New("login").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Memory Cloud — Authorize Access</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #e2e8f0;
        }
        .card {
            background: #1e293b;
            border: 1px solid #334155;
            border-radius: 16px;
            padding: 2.5rem;
            width: 100%;
            max-width: 400px;
            box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
        }
        .logo {
            text-align: center;
            margin-bottom: 1.5rem;
        }
        .logo h1 {
            font-size: 1.5rem;
            font-weight: 600;
            color: #f1f5f9;
        }
        .logo p {
            color: #94a3b8;
            font-size: 0.9rem;
            margin-top: 0.25rem;
        }
        .client-info {
            background: #0f172a;
            border-radius: 8px;
            padding: 0.75rem 1rem;
            margin-bottom: 1.5rem;
            font-size: 0.85rem;
            color: #94a3b8;
        }
        .client-info strong { color: #e2e8f0; }
        .error {
            background: #450a0a;
            border: 1px solid #7f1d1d;
            color: #fca5a5;
            border-radius: 8px;
            padding: 0.75rem 1rem;
            margin-bottom: 1rem;
            font-size: 0.85rem;
        }
        label {
            display: block;
            font-size: 0.85rem;
            color: #94a3b8;
            margin-bottom: 0.5rem;
        }
        input[type="password"] {
            width: 100%;
            padding: 0.75rem 1rem;
            background: #0f172a;
            border: 1px solid #334155;
            border-radius: 8px;
            color: #f1f5f9;
            font-size: 1rem;
            outline: none;
            transition: border-color 0.2s;
        }
        input[type="password"]:focus {
            border-color: #3b82f6;
        }
        button {
            width: 100%;
            padding: 0.75rem;
            background: #3b82f6;
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 1rem;
            font-weight: 500;
            cursor: pointer;
            margin-top: 1rem;
            transition: background 0.2s;
        }
        button:hover { background: #2563eb; }
    </style>
</head>
<body>
    <div class="card">
        <div class="logo">
            <h1>Memory Cloud</h1>
            <p>Authorize Access</p>
        </div>
        {{if .ClientName}}
        <div class="client-info">
            <strong>{{.ClientName}}</strong> is requesting access to your MCP servers.
        </div>
        {{end}}
        {{if .Error}}
        <div class="error">{{.Error}}</div>
        {{end}}
        <form method="POST" action="/oauth/authorize">
            <input type="hidden" name="response_type" value="{{.ResponseType}}">
            <input type="hidden" name="client_id" value="{{.ClientID}}">
            <input type="hidden" name="redirect_uri" value="{{.RedirectURI}}">
            <input type="hidden" name="state" value="{{.State}}">
            <input type="hidden" name="code_challenge" value="{{.CodeChallenge}}">
            <input type="hidden" name="code_challenge_method" value="{{.CodeChallengeMethod}}">
            <input type="hidden" name="resource" value="{{.Resource}}">
            <label for="password">Password</label>
            <input type="password" id="password" name="password" placeholder="Enter authorization password" autofocus required>
            <button type="submit">Authorize</button>
        </form>
    </div>
</body>
</html>`))

type LoginPageData struct {
	ClientName          string
	Error               string
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

// ── Server ──────────────────────────────────────────────

type Server struct {
	config Config
	store  *Store
	mux    *http.ServeMux
}

func NewServer(config Config) *Server {
	s := &Server{
		config: config,
		store:  NewStore(),
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.handleProtectedResource)
	s.mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.handleAuthorizationServer)
	s.mux.HandleFunc("POST /oauth/register", s.handleRegister)
	s.mux.HandleFunc("GET /oauth/authorize", s.handleAuthorizeGET)
	s.mux.HandleFunc("POST /oauth/authorize", s.handleAuthorizePOST)
	s.mux.HandleFunc("POST /oauth/token", s.handleToken)
}

// ── Task 4.1: Metadata endpoints ────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleProtectedResource(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"resource":              s.config.MCPResourceURL,
		"authorization_servers": []string{s.config.OAuthServerBaseURL},
	})
}

func (s *Server) handleAuthorizationServer(w http.ResponseWriter, r *http.Request) {
	base := s.config.OAuthServerBaseURL
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"issuer":                              base,
		"authorization_endpoint":              base + "/oauth/authorize",
		"token_endpoint":                      base + "/oauth/token",
		"registration_endpoint":               base + "/oauth/register",
		"response_types_supported":            []string{"code"},
		"grant_types_supported":               []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":    []string{"S256"},
	})
}

// ── Task 4.2: Dynamic Client Registration ───────────────

type RegisterRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if len(req.RedirectURIs) == 0 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uris is required")
		return
	}

	clientID := uuid.New().String()

	// Default grant_types and response_types if not provided
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = []string{"authorization_code"}
	}
	if len(req.ResponseTypes) == 0 {
		req.ResponseTypes = []string{"code"}
	}
	if req.TokenEndpointAuthMethod == "" {
		req.TokenEndpointAuthMethod = "none"
	}

	client := &Client{
		ClientID:                clientID,
		ClientName:              req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
	}

	s.store.mu.Lock()
	s.store.clients[clientID] = client
	s.store.mu.Unlock()

	log.Printf("[DCR] Registered client: %s (id: %s, redirects: %v)", req.ClientName, clientID, req.RedirectURIs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(client)
}

// ── Task 4.3: Authorization endpoint ────────────────────

func (s *Server) handleAuthorizeGET(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")

	// Validate client_id
	s.store.mu.RLock()
	client, exists := s.store.clients[clientID]
	s.store.mu.RUnlock()

	if !exists {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unknown client_id")
		return
	}

	// Validate redirect_uri matches registered
	if !s.validRedirectURI(client, redirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri does not match registered URIs")
		return
	}

	data := LoginPageData{
		ClientName:          client.ClientName,
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
		Resource:            r.URL.Query().Get("resource"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	loginTemplate.Execute(w, data)
}

func (s *Server) handleAuthorizePOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Invalid form data")
		return
	}

	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")
	resource := r.FormValue("resource")
	password := r.FormValue("password")
	responseType := r.FormValue("response_type")

	// Validate client
	s.store.mu.RLock()
	client, exists := s.store.clients[clientID]
	s.store.mu.RUnlock()

	if !exists {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unknown client_id")
		return
	}

	if !s.validRedirectURI(client, redirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri does not match registered URIs")
		return
	}

	// Validate password
	if password != s.config.OAuthAuthorizePassword {
		log.Printf("[AUTH] Invalid password attempt for client %s", clientID)
		data := LoginPageData{
			ClientName:          client.ClientName,
			Error:               "Invalid password",
			ResponseType:        responseType,
			ClientID:            clientID,
			RedirectURI:         redirectURI,
			State:               state,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: codeChallengeMethod,
			Resource:            resource,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		loginTemplate.Execute(w, data)
		return
	}

	// Generate auth code
	code := randomHex(32) // 64 hex chars

	s.store.mu.Lock()
	s.store.authCodes[code] = &AuthCode{
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		CodeChallenge: codeChallenge,
		Resource:      resource,
		CreatedAt:     time.Now(),
	}
	s.store.mu.Unlock()

	log.Printf("[AUTH] Auth code issued for client %s", clientID)

	// Redirect to client
	redirectTo := fmt.Sprintf("%s?code=%s", redirectURI, code)
	if state != "" {
		redirectTo += "&state=" + state
	}

	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (s *Server) validRedirectURI(client *Client, uri string) bool {
	for _, registered := range client.RedirectURIs {
		if registered == uri {
			return true
		}
	}
	return false
}

// ── Task 4.4: Token endpoint ────────────────────────────

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Invalid form data")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleTokenAuthCode(w, r)
	case "refresh_token":
		s.handleTokenRefresh(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("Unsupported grant_type: %s", grantType))
	}
}

func (s *Server) handleTokenAuthCode(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")
	clientID := r.FormValue("client_id")
	codeVerifier := r.FormValue("code_verifier")

	if code == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Missing required parameters")
		return
	}

	// Look up and consume auth code (one-time use)
	s.store.mu.Lock()
	authCode, exists := s.store.authCodes[code]
	if exists {
		delete(s.store.authCodes, code) // One-time use
	}
	s.store.mu.Unlock()

	if !exists {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Invalid or expired authorization code")
		return
	}

	// Check expiry (5 minutes)
	if time.Since(authCode.CreatedAt) > 5*time.Minute {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Authorization code expired")
		return
	}

	// Validate client_id matches
	if authCode.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id does not match")
		return
	}

	// Validate redirect_uri matches
	if authCode.RedirectURI != redirectURI {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match")
		return
	}

	// Validate PKCE
	if authCode.CodeChallenge != "" {
		if codeVerifier == "" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "code_verifier required for PKCE")
			return
		}
		if !verifyPKCE(codeVerifier, authCode.CodeChallenge) {
			writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
			return
		}
	}

	// Generate refresh token
	refreshToken := randomHex(32) // 64 hex chars

	s.store.mu.Lock()
	s.store.refreshTokens[refreshToken] = &RefreshToken{
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}
	s.store.mu.Unlock()

	log.Printf("[TOKEN] Access token issued for client %s (auth_code grant)", clientID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  s.config.MCPBearerToken,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": refreshToken,
	})
}

func (s *Server) handleTokenRefresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")
	clientID := r.FormValue("client_id")

	if refreshToken == "" || clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Missing required parameters")
		return
	}

	// Look up and consume refresh token
	s.store.mu.Lock()
	rt, exists := s.store.refreshTokens[refreshToken]
	if exists {
		delete(s.store.refreshTokens, refreshToken) // Rotate token
	}
	s.store.mu.Unlock()

	if !exists {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "Invalid refresh token")
		return
	}

	// Validate client_id
	if rt.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "client_id does not match")
		return
	}

	// Issue new refresh token
	newRefreshToken := randomHex(32)

	s.store.mu.Lock()
	s.store.refreshTokens[newRefreshToken] = &RefreshToken{
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}
	s.store.mu.Unlock()

	log.Printf("[TOKEN] Access token refreshed for client %s", clientID)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  s.config.MCPBearerToken,
		"token_type":    "Bearer",
		"expires_in":    86400,
		"refresh_token": newRefreshToken,
	})
}

// ── Logging middleware ───────────────────────────────────

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// ── Main ────────────────────────────────────────────────

func main() {
	config := loadConfig()

	if config.OAuthAuthorizePassword == "" {
		log.Fatal("FATAL: OAUTH_AUTHORIZE_PASSWORD is required")
	}
	if config.MCPBearerToken == "" {
		log.Fatal("FATAL: MCP_BEARER_TOKEN is required")
	}

	server := NewServer(config)

	addr := ":" + config.Port
	log.Printf("OAuth server starting on %s", addr)
	log.Printf("  Base URL:     %s", config.OAuthServerBaseURL)
	log.Printf("  Resource URL: %s", config.MCPResourceURL)

	httpServer := &http.Server{
		Addr:         addr,
		Handler:      loggingMiddleware(server.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
