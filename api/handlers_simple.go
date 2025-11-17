package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Vova4o/VPSSetup/pkg/provider"
)

// handleGetRegions returns available regions
func (s *Server) handleGetRegions(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	var vpsProvider provider.VPSProvider
	switch profile.VPS.Provider {
	case "digitalocean":
		vpsProvider = provider.NewDigitalOceanProvider(profile.VPS.APIKey)
	default:
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := withTimeout(r.Context(), timeoutShort)
	defer cancel()

	regions, err := vpsProvider.ListRegions(ctx)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, regions)
}

// handleGetSizes returns available instance sizes
func (s *Server) handleGetSizes(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	var vpsProvider provider.VPSProvider
	switch profile.VPS.Provider {
	case "digitalocean":
		vpsProvider = provider.NewDigitalOceanProvider(profile.VPS.APIKey)
	default:
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("unsupported provider"))
		return
	}

	ctx, cancel := withTimeout(r.Context(), timeoutShort)
	defer cancel()

	sizes, err := vpsProvider.ListSizes(ctx)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Filter by region if specified
	region := r.URL.Query().Get("region")
	if region != "" {
		filteredSizes := make([]provider.Size, 0)
		for _, size := range sizes {
			// Check if size is available in the requested region
			if len(size.Regions) == 0 || containsString(size.Regions, region) {
				filteredSizes = append(filteredSizes, size)
			}
		}
		sizes = filteredSizes
	}

	s.respondSuccess(w, sizes)
}

func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// handleGetConfig returns current configuration
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, s.config)
}

// handleUpdateConfig updates configuration
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if err := parseJSON(r, s.config); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	if err := s.config.Save("./config.yaml"); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": "Configuration updated successfully",
	})
}

// Note: NGINX and SSL handlers are in handlers_nginx_ssl.go
// DNS, Deploy, Logs handlers are stubs below (CLI recommended for now)

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "Deployment available via CLI: vpssetup upload",
		"tip":     "Use docker-compose, static, or standalone deployment types",
	})
}

func (s *Server) handleDNSList(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "DNS list available via CLI: vpssetup dns list",
	})
}

func (s *Server) handleDNSCreate(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "DNS create available via CLI: vpssetup dns create",
	})
}

func (s *Server) handleDNSUpdate(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "DNS update available via CLI: vpssetup dns update",
	})
}

func (s *Server) handleDNSRemove(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "DNS remove available via CLI: vpssetup dns remove",
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Get log type from URL path variable
	logType := r.URL.Query().Get("type")
	if logType == "" {
		// Try to get from path
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 2 {
			logType = parts[len(parts)-1]
		}
	}

	lines := 100
	if linesParam := r.URL.Query().Get("lines"); linesParam != "" {
		fmt.Sscanf(linesParam, "%d", &lines)
	}

	// For now, just return a sample message showing what would be fetched
	s.respondSuccess(w, map[string]string{
		"logs": fmt.Sprintf("Logs for %s (last %d lines) would appear here.\n\nTo view actual logs, SSH to your VPS at %s:\nssh %s@%s\n\nThen run:\n- NGINX access: tail -n %d /var/log/nginx/access.log\n- NGINX error: tail -n %d /var/log/nginx/error.log\n- System: journalctl -n %d\n- SSH: tail -n %d /var/log/auth.log\n- Fail2ban: tail -n %d /var/log/fail2ban.log",
			logType, lines, profile.VPS.PublicIP, profile.SSH.User, profile.VPS.PublicIP, lines, lines, lines, lines, lines),
	})
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	s.respondSuccess(w, map[string]string{
		"message": "Log streaming available via CLI: vpssetup logs --follow",
	})
}
