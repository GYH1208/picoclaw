package api

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/web/backend/middleware"
	"github.com/sipeed/picoclaw/web/backend/launcherconfig"
)

func TestGetLauncherConfigUsesRuntimeFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)
	h.SetServerOptions(19999, true, false, []string{"192.168.1.0/24"})

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/launcher-config", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got launcherConfigPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Port != 19999 || !got.Public {
		t.Fatalf("response = %+v, want port=19999 public=true", got)
	}
	if len(got.AllowedCIDRs) != 1 || got.AllowedCIDRs[0] != "192.168.1.0/24" {
		t.Fatalf("response allowed_cidrs = %v, want [192.168.1.0/24]", got.AllowedCIDRs)
	}
}

func TestPutLauncherConfigPersists(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/system/launcher-config",
		strings.NewReader(`{"port":18080,"public":true,"allowed_cidrs":["192.168.1.0/24"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	path := launcherconfig.PathForAppConfig(configPath)
	cfg, err := launcherconfig.Load(path, launcherconfig.Default())
	if err != nil {
		t.Fatalf("launcherconfig.Load() error = %v", err)
	}
	if cfg.Port != 18080 || !cfg.Public {
		t.Fatalf("saved config = %+v, want port=18080 public=true", cfg)
	}
	if len(cfg.AllowedCIDRs) != 1 || cfg.AllowedCIDRs[0] != "192.168.1.0/24" {
		t.Fatalf("saved config allowed_cidrs = %v, want [192.168.1.0/24]", cfg.AllowedCIDRs)
	}
}

func TestPutLauncherConfigRejectsInvalidPort(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/system/launcher-config",
		strings.NewReader(`{"port":70000,"public":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPutLauncherConfigRejectsInvalidCIDR(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/system/launcher-config",
		strings.NewReader(`{"port":18080,"public":false,"allowed_cidrs":["bad-cidr"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPostLauncherTokenPersistsForNextRestart(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	h.SetDashboardAuthState(key, "old-token", false)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/system/launcher-token",
		strings.NewReader(`{"current_token":"old-token","new_token":"new-token-123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	cfg, err := launcherconfig.Load(launcherconfig.PathForAppConfig(configPath), launcherconfig.Default())
	if err != nil {
		t.Fatalf("launcherconfig.Load() error = %v", err)
	}
	if cfg.DashboardToken != "new-token-123" {
		t.Fatalf("dashboard_token = %q, want %q", cfg.DashboardToken, "new-token-123")
	}
	if got := h.DashboardTokenValue(); got != "new-token-123" {
		t.Fatalf("runtime token = %q, want %q", got, "new-token-123")
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != middleware.LauncherDashboardCookieName {
		t.Fatalf("cookies = %#v", cookies)
	}
	if cookies[0].Value != h.DashboardSessionCookieValue() {
		t.Fatalf("cookie value = %q, want %q", cookies[0].Value, h.DashboardSessionCookieValue())
	}
}

func TestPostLauncherTokenRejectsWhenEnvManaged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	h := NewHandler(configPath)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	h.SetDashboardAuthState(key, "old-token", true)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/system/launcher-token",
		strings.NewReader(`{"current_token":"old-token","new_token":"new-token-123"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}
