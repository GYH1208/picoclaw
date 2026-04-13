package api

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/web/backend/middleware"
	"github.com/sipeed/picoclaw/web/backend/launcherconfig"
)

type launcherConfigPayload struct {
	Port         int      `json:"port"`
	Public       bool     `json:"public"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
}

type updateLauncherTokenPayload struct {
	CurrentToken string `json:"current_token"`
	NewToken     string `json:"new_token"`
}

type updateLauncherTokenResponse struct {
	Status         string `json:"status"`
	ApplyOnRestart bool   `json:"apply_on_restart"`
}

func (h *Handler) registerLauncherConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/system/launcher-config", h.handleGetLauncherConfig)
	mux.HandleFunc("PUT /api/system/launcher-config", h.handleUpdateLauncherConfig)
	mux.HandleFunc("POST /api/system/launcher-token", h.handleUpdateLauncherToken)
}

func (h *Handler) launcherConfigPath() string {
	return launcherconfig.PathForAppConfig(h.configPath)
}

func (h *Handler) launcherFallbackConfig() launcherconfig.Config {
	port := h.serverPort
	if port <= 0 {
		port = launcherconfig.DefaultPort
	}
	return launcherconfig.Config{
		Port:           port,
		Public:         h.serverPublic,
		AllowedCIDRs:   append([]string(nil), h.serverCIDRs...),
		DashboardToken: h.DashboardTokenValue(),
	}
}

func (h *Handler) loadLauncherConfig() (launcherconfig.Config, error) {
	return launcherconfig.Load(h.launcherConfigPath(), h.launcherFallbackConfig())
}

func (h *Handler) handleGetLauncherConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.loadLauncherConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load launcher config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(launcherConfigPayload{
		Port:         cfg.Port,
		Public:       cfg.Public,
		AllowedCIDRs: append([]string(nil), cfg.AllowedCIDRs...),
	})
}

func (h *Handler) handleUpdateLauncherConfig(w http.ResponseWriter, r *http.Request) {
	var payload launcherConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	cfg := launcherconfig.Config{
		Port:           payload.Port,
		Public:         payload.Public,
		AllowedCIDRs:   append([]string(nil), payload.AllowedCIDRs...),
		DashboardToken: h.DashboardTokenValue(),
	}
	if err := launcherconfig.Validate(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := launcherconfig.Save(h.launcherConfigPath(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save launcher config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(launcherConfigPayload{
		Port:         cfg.Port,
		Public:       cfg.Public,
		AllowedCIDRs: append([]string(nil), cfg.AllowedCIDRs...),
	})
}

func (h *Handler) handleUpdateLauncherToken(w http.ResponseWriter, r *http.Request) {
	_, _, envSet := h.dashboardAuthState()
	if envSet {
		http.Error(w, "launcher token is managed by PICOCLAW_LAUNCHER_TOKEN and cannot be changed here", http.StatusConflict)
		return
	}

	var payload updateLauncherTokenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	currentToken := strings.TrimSpace(payload.CurrentToken)
	newToken := strings.TrimSpace(payload.NewToken)
	if newToken == "" {
		http.Error(w, "new token is required", http.StatusBadRequest)
		return
	}
	if len(newToken) < 6 {
		http.Error(w, "new token must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if !h.verifyCurrentDashboardToken(currentToken) {
		http.Error(w, "current token is invalid", http.StatusUnauthorized)
		return
	}
	if len(currentToken) == len(newToken) &&
		subtle.ConstantTimeCompare([]byte(currentToken), []byte(newToken)) == 1 {
		http.Error(w, "new token must differ from current token", http.StatusBadRequest)
		return
	}

	cfg, err := h.loadLauncherConfig()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load launcher config: %v", err), http.StatusInternalServerError)
		return
	}
	cfg.DashboardToken = newToken
	if err := launcherconfig.Save(h.launcherConfigPath(), cfg); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save launcher config: %v", err), http.StatusInternalServerError)
		return
	}
	h.SetDashboardAuthState(h.dashboardSigningKeyCopy(), newToken, envSet)
	middleware.SetLauncherDashboardSessionCookie(w, r, h.DashboardSessionCookieValue(), nil)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updateLauncherTokenResponse{
		Status:         "ok",
		ApplyOnRestart: false,
	})
}
