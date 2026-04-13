package launcherconfig

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FileName is the launcher-specific settings file name.
	FileName = "launcher-config.json"
	// DefaultPort is the default port for the web launcher.
	DefaultPort = 18800

	// dashboardSigningKeyBytes is the HMAC-SHA256 key size (256 bits).
	dashboardSigningKeyBytes = 32
	// defaultDashboardToken is used when PICOCLAW_LAUNCHER_TOKEN is not set.
	defaultDashboardToken = "000000"
)

// Config stores launch parameters for the web backend service.
type Config struct {
	Port           int      `json:"port"`
	Public         bool     `json:"public"`
	AllowedCIDRs   []string `json:"allowed_cidrs,omitempty"`
	DashboardToken string   `json:"dashboard_token,omitempty"`
}

// Default returns default launcher settings.
func Default() Config {
	return Config{Port: DefaultPort, Public: false}
}

// Validate checks if launcher settings are valid.
func Validate(cfg Config) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port %d is out of range (1-65535)", cfg.Port)
	}
	for _, cidr := range cfg.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR %q", cidr)
		}
	}
	return nil
}

// EnsureDashboardSecrets returns signing key bytes and the effective dashboard token for this
// process. The signing key is freshly random each call; the token comes from the environment
// variable PICOCLAW_LAUNCHER_TOKEN when set, otherwise a default token.
func EnsureDashboardSecrets(configToken string) (effectiveToken string, signingKey []byte, newRandomDashboardToken bool, err error) {
	signingKey = make([]byte, dashboardSigningKeyBytes)
	if _, err = rand.Read(signingKey); err != nil {
		return "", nil, false, err
	}

	effectiveToken = strings.TrimSpace(os.Getenv("PICOCLAW_LAUNCHER_TOKEN"))
	if effectiveToken != "" {
		return effectiveToken, signingKey, false, nil
	}
	effectiveToken = strings.TrimSpace(configToken)
	if effectiveToken != "" {
		return effectiveToken, signingKey, false, nil
	}
	return defaultDashboardToken, signingKey, true, nil
}

// NormalizeCIDRs trims entries, removes empty values, and deduplicates CIDRs.
func NormalizeCIDRs(cidrs []string) []string {
	if len(cidrs) == 0 {
		return nil
	}
	out := make([]string, 0, len(cidrs))
	seen := make(map[string]struct{}, len(cidrs))
	for _, raw := range cidrs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PathForAppConfig returns launcher-config path near the app config file.
func PathForAppConfig(appConfigPath string) string {
	dir := filepath.Dir(appConfigPath)
	if dir == "" || dir == "." {
		dir = "."
	}
	return filepath.Join(dir, FileName)
}

// Load reads launcher settings; fallback is returned when file does not exist.
func Load(path string, fallback Config) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return Config{}, err
	}

	cfg := fallback
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	cfg.AllowedCIDRs = NormalizeCIDRs(cfg.AllowedCIDRs)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes launcher settings to disk.
func Save(path string, cfg Config) error {
	cfg.AllowedCIDRs = NormalizeCIDRs(cfg.AllowedCIDRs)
	if err := Validate(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
