package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Vova4o/VPSSetup/internal/config"
	"gopkg.in/yaml.v3"
)

// InitConfigRequest represents initialization request
type InitConfigRequest struct {
	Profile     string `json:"profile"`
	VPSProvider string `json:"vps_provider"`
	VPSAPIToken string `json:"vps_api_token"`
	DNSProvider string `json:"dns_provider"`
	DNSAPIToken string `json:"dns_api_token"`
	DNSAPIURL   string `json:"dns_api_url"`
	Domain      string `json:"domain"`
	Subdomain   string `json:"subdomain"`
	SSHUser     string `json:"ssh_user"`
	SSHKeyPath  string `json:"ssh_key_path"`
	SSLEmail    string `json:"ssl_email"`
	SSLProvider string `json:"ssl_provider"`
}

// handleInitConfig handles configuration initialization
func (s *Server) handleInitConfig(w http.ResponseWriter, r *http.Request) {
	var req InitConfigRequest
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Validate required fields
	if req.Profile == "" {
		req.Profile = "default"
	}
	if req.SSLEmail == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("ssl_email is required"))
		return
	}
	if req.VPSAPIToken == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("vps_api_token is required"))
		return
	}

	// Create profile
	profile := &config.Profile{
		VPS: config.VPSConfig{
			Provider: req.VPSProvider,
			APIKey:   req.VPSAPIToken,
		},
		Domain: config.DomainConfig{
			Name:        req.Domain,
			DNSProvider: req.DNSProvider,
			DNSAPIKey:   req.DNSAPIToken,
		},
		SSH: config.SSHConfig{
			User:    req.SSHUser,
			KeyPath: req.SSHKeyPath,
		},
		SSL: config.SSLConfig{
			Email:     req.SSLEmail,
			Enable:    true,
			AutoRenew: true,
		},
	}

	// Add subdomain if provided
	if req.Subdomain != "" {
		profile.Domain.Name = req.Subdomain + "." + req.Domain
	}

	// Set default SSH user if not provided
	if profile.SSH.User == "" {
		profile.SSH.User = "root"
	}

	// Create config structure
	configData := map[string]interface{}{
		"default_profile": req.Profile,
		"profiles": map[string]interface{}{
			req.Profile: profile,
		},
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(configData)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to create config: %w", err))
		return
	}

	// Write to file
	configPath := "./config.yaml"
	if err := os.WriteFile(configPath, yamlData, 0o644); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to write config file: %w", err))
		return
	}

	// Reload server config
	newConfig, err := config.Load(configPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to reload config: %w", err))
		return
	}
	s.config = newConfig

	s.respondSuccess(w, map[string]interface{}{
		"message": "Configuration initialized successfully",
		"profile": req.Profile,
		"config":  profile,
	})
}

// handleCheckConfig checks if configuration exists
func (s *Server) handleCheckConfig(w http.ResponseWriter, r *http.Request) {
	exists := false
	hasProfiles := false

	if _, err := os.Stat("./config.yaml"); err == nil {
		exists = true
		if s.config != nil && len(s.config.Profiles) > 0 {
			hasProfiles = true
		}
	}

	s.respondSuccess(w, map[string]interface{}{
		"exists":       exists,
		"has_profiles": hasProfiles,
		"profiles":     s.config.Profiles,
	})
}
