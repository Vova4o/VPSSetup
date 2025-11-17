package api

import (
	"fmt"
	"net/http"

	"github.com/Vova4o/VPSSetup/internal/connection"
	inginx "github.com/Vova4o/VPSSetup/internal/nginx"
	"github.com/Vova4o/VPSSetup/internal/ssl"
	"github.com/Vova4o/VPSSetup/pkg/nginx"
	"github.com/Vova4o/VPSSetup/pkg/utils"
)

// sshAdapter adapts SSHClient to nginx.SSHExecutor interface
type sshAdapter struct {
	client *connection.SSHClient
}

func (a *sshAdapter) RunCommand(cmd string) (string, error) {
	return a.client.ExecuteCommand(cmd)
}

// NginxSetupRequest represents nginx setup request
type NginxSetupRequest struct {
	Domain   string `json:"domain"`
	Type     string `json:"type"` // static, proxy, php
	RootPath string `json:"root_path,omitempty"`
	ProxyTo  string `json:"proxy_to,omitempty"`
}

// SSLInstallRequest represents SSL install request
type SSLInstallRequest struct {
	Domain string `json:"domain"`
	Email  string `json:"email"`
}

// handleNginxSetup handles nginx configuration setup
func (s *Server) handleNginxSetup(w http.ResponseWriter, r *http.Request) {
	var req NginxSetupRequest
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	sshClient, err := connection.NewSSHClient(profile.VPS.PublicIP, profile.SSH.User, keyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := sshClient.Connect(); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	defer sshClient.Close()

	adapter := &sshAdapter{client: sshClient}
	nginxService := inginx.NewService(adapter)

	var nginxCfg *nginx.Config
	switch req.Type {
	case "static":
		nginxCfg = nginx.NewConfig(req.Domain, nginx.ConfigTypeStatic)
		nginxCfg.RootPath = req.RootPath
	case "proxy":
		nginxCfg = nginx.NewConfig(req.Domain, nginx.ConfigTypeReverseProxy)
		if req.ProxyTo != "" {
			fmt.Sscanf(req.ProxyTo, "localhost:%d", &nginxCfg.ProxyPort)
		}
	case "php":
		nginxCfg = nginx.NewConfig(req.Domain, nginx.ConfigTypePHP)
		nginxCfg.RootPath = req.RootPath
	default:
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("invalid nginx type: %s", req.Type))
		return
	}

	configContent, err := nginxCfg.Generate()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := nginxService.Deploy(req.Domain, configContent, req.RootPath, req.Type); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": fmt.Sprintf("Nginx configured for %s", req.Domain),
	})
}

// handleNginxTest handles nginx configuration test
func (s *Server) handleNginxTest(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to expand SSH key path: %w", err))
		return
	}

	sshClient, err := connection.NewSSHClient(profile.VPS.PublicIP, profile.SSH.User, keyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to create SSH client: %w", err))
		return
	}

	if err := sshClient.Connect(); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("SSH connection failed (check VPS IP, SSH user, and key path in config): %w", err))
		return
	}
	defer sshClient.Close()

	output, err := sshClient.ExecuteCommand("sudo nginx -t")
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("nginx test failed: %s", output))
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": "Nginx configuration is valid",
		"output":  output,
	})
}

// handleNginxReload handles nginx reload
func (s *Server) handleNginxReload(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	sshClient, err := connection.NewSSHClient(profile.VPS.PublicIP, profile.SSH.User, keyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := sshClient.Connect(); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	defer sshClient.Close()

	adapter := &sshAdapter{client: sshClient}
	nginxService := inginx.NewService(adapter)

	if err := nginxService.Reload(); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": "Nginx reloaded successfully",
	})
}

// handleSSLInstall handles SSL certificate installation
func (s *Server) handleSSLInstall(w http.ResponseWriter, r *http.Request) {
	var req SSLInstallRequest
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	sshClient, err := connection.NewSSHClient(profile.VPS.PublicIP, profile.SSH.User, keyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := sshClient.Connect(); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	defer sshClient.Close()

	adapter := &sshAdapter{client: sshClient}
	sslService := ssl.NewService(adapter)

	sslConfig := ssl.Config{
		Domain: req.Domain,
		Email:  req.Email,
	}

	if err := sslService.Install(sslConfig); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": fmt.Sprintf("SSL certificate installed for %s", req.Domain),
	})
}

// handleSSLRenew handles SSL certificate renewal
func (s *Server) handleSSLRenew(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	sshClient, err := connection.NewSSHClient(profile.VPS.PublicIP, profile.SSH.User, keyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	if err := sshClient.Connect(); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	defer sshClient.Close()

	adapter := &sshAdapter{client: sshClient}
	sslService := ssl.NewService(adapter)

	if err := sslService.Renew(req.Domain); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": fmt.Sprintf("SSL certificate renewed for %s", req.Domain),
	})
}
