package api

import (
	"crypto/subtle"
	"net/http"
	"sync"

	"github.com/sipeed/picoclaw/web/backend/middleware"
	"github.com/sipeed/picoclaw/web/backend/launcherconfig"
)

// Handler serves HTTP API requests.
type Handler struct {
	configPath           string
	serverPort           int
	serverPublic         bool
	serverPublicExplicit bool
	serverCIDRs          []string
	dashboardAuthMu      sync.RWMutex
	dashboardSigningKey  []byte
	dashboardToken       string
	dashboardTokenEnvSet bool
	oauthMu              sync.Mutex
	oauthFlows           map[string]*oauthFlow
	oauthState           map[string]string
	weixinMu             sync.Mutex
	weixinFlows          map[string]*weixinFlow
	wecomMu              sync.Mutex
	wecomFlows           map[string]*wecomFlow
}

// NewHandler creates an instance of the API handler.
func NewHandler(configPath string) *Handler {
	return &Handler{
		configPath:  configPath,
		serverPort:  launcherconfig.DefaultPort,
		oauthFlows:  make(map[string]*oauthFlow),
		oauthState:  make(map[string]string),
		weixinFlows: make(map[string]*weixinFlow),
		wecomFlows:  make(map[string]*wecomFlow),
	}
}

// SetServerOptions stores current backend listen options for fallback behavior.
func (h *Handler) SetServerOptions(port int, public bool, publicExplicit bool, allowedCIDRs []string) {
	h.serverPort = port
	h.serverPublic = public
	h.serverPublicExplicit = publicExplicit
	h.serverCIDRs = append([]string(nil), allowedCIDRs...)
}

// SetDashboardAuthState stores launcher dashboard auth state for token management APIs.
func (h *Handler) SetDashboardAuthState(signingKey []byte, token string, envSet bool) {
	h.dashboardAuthMu.Lock()
	defer h.dashboardAuthMu.Unlock()
	h.dashboardSigningKey = append([]byte(nil), signingKey...)
	h.dashboardToken = token
	h.dashboardTokenEnvSet = envSet
}

func (h *Handler) dashboardAuthState() (token string, sessionCookie string, envSet bool) {
	h.dashboardAuthMu.RLock()
	defer h.dashboardAuthMu.RUnlock()
	return h.dashboardToken, middleware.SessionCookieValue(h.dashboardSigningKey, h.dashboardToken), h.dashboardTokenEnvSet
}

func (h *Handler) verifyCurrentDashboardToken(input string) bool {
	h.dashboardAuthMu.RLock()
	token := h.dashboardToken
	h.dashboardAuthMu.RUnlock()
	in := input
	if len(in) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(in), []byte(token)) == 1
}

func (h *Handler) DashboardTokenValue() string {
	h.dashboardAuthMu.RLock()
	defer h.dashboardAuthMu.RUnlock()
	return h.dashboardToken
}

func (h *Handler) DashboardSessionCookieValue() string {
	h.dashboardAuthMu.RLock()
	defer h.dashboardAuthMu.RUnlock()
	return middleware.SessionCookieValue(h.dashboardSigningKey, h.dashboardToken)
}

func (h *Handler) dashboardSigningKeyCopy() []byte {
	h.dashboardAuthMu.RLock()
	defer h.dashboardAuthMu.RUnlock()
	return append([]byte(nil), h.dashboardSigningKey...)
}

// RegisterRoutes binds all API endpoint handlers to the ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Config CRUD
	h.registerConfigRoutes(mux)

	// Pico Channel (WebSocket chat)
	h.registerPicoRoutes(mux)

	// Gateway process lifecycle
	h.registerGatewayRoutes(mux)

	// Session history
	h.registerSessionRoutes(mux)

	// OAuth login and credential management
	h.registerOAuthRoutes(mux)

	// Model list management
	h.registerModelRoutes(mux)

	// Channel catalog (for frontend navigation/config pages)
	h.registerChannelRoutes(mux)

	// Skills and tools support/actions
	h.registerSkillRoutes(mux)
	h.registerToolRoutes(mux)

	// OS startup / launch-at-login
	h.registerStartupRoutes(mux)

	// Launcher service parameters (port/public)
	h.registerLauncherConfigRoutes(mux)

	// Runtime build/version metadata
	h.registerVersionRoutes(mux)

	// WeChat QR login flow
	h.registerWeixinRoutes(mux)

	// WeCom QR login flow
	h.registerWecomRoutes(mux)
}

// Shutdown gracefully shuts down the handler, stopping the gateway if it was started by this handler.
func (h *Handler) Shutdown() {
	h.StopGateway()
}
