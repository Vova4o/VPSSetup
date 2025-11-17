package api

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Vova4o/VPSSetup/internal/connection"
	"github.com/Vova4o/VPSSetup/internal/hardening"
	"github.com/Vova4o/VPSSetup/internal/setup"
	"github.com/Vova4o/VPSSetup/pkg/provider"
	"github.com/Vova4o/VPSSetup/pkg/utils"
)

// VPSSetupRequest represents VPS setup request
type VPSSetupRequest struct {
	Region string `json:"region"`
	Size   string `json:"size"`
	Name   string `json:"name"`
}

// handleVPSSetup handles VPS creation
func (s *Server) handleVPSSetup(w http.ResponseWriter, r *http.Request) {
	var req VPSSetupRequest
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Create provider
	var vpsProvider provider.VPSProvider
	switch profile.VPS.Provider {
	case "digitalocean":
		vpsProvider = provider.NewDigitalOceanProvider(profile.VPS.APIKey)
	default:
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("unsupported provider: %s", profile.VPS.Provider))
		return
	}

	// Setup VPS
	ctx, cancel := withTimeout(r.Context(), timeoutLong)
	defer cancel()

	keyPath, err := utils.ExpandPath(profile.SSH.KeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Read public key content
	pubKeyPath := keyPath + ".pub"
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to read SSH public key from %s: %w", pubKeyPath, err))
		return
	}
	pubKeyContent := string(pubKeyBytes)

	// Cast to DigitalOceanProvider
	doProvider, ok := vpsProvider.(*provider.DigitalOceanProvider)
	if !ok {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("only DigitalOcean provider is supported for setup"))
		return
	}

	setupService := setup.NewService(doProvider)
	instance, err := setupService.SetupVPS(ctx, setup.Config{
		Name:         req.Name,
		Region:       req.Region,
		Size:         req.Size,
		SSHUser:      profile.SSH.User,
		SSHKeyPath:   keyPath,
		SSHPublicKey: pubKeyContent,
	})
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Update config
	profile.VPS.InstanceID = instance.ID
	profile.VPS.PublicIP = instance.PublicIP
	profile.VPS.Name = instance.Name
	profile.VPS.Region = instance.Region
	profile.VPS.Size = instance.Size
	profile.VPS.CreatedAt = instance.CreatedAt

	if err := s.config.Save("./config.yaml"); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]interface{}{
		"instance": instance,
		"message":  "VPS created successfully",
	})
}

// handleVPSStatus handles VPS status check
func (s *Server) handleVPSStatus(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	if profile.VPS.InstanceID == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("no VPS configured"))
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

	instance, err := vpsProvider.GetInstance(ctx, profile.VPS.InstanceID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, instance)
}

// handleVPSDestroy handles VPS destruction
func (s *Server) handleVPSDestroy(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	if profile.VPS.InstanceID == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("no VPS configured"))
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

	ctx, cancel := withTimeout(r.Context(), timeoutLong)
	defer cancel()

	if err := vpsProvider.DeleteInstance(ctx, profile.VPS.InstanceID); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	// Clear VPS info from config
	cleanConfig := r.URL.Query().Get("clean_config") == "true"
	if cleanConfig {
		profile.VPS.InstanceID = ""
		profile.VPS.PublicIP = ""
		profile.VPS.Name = ""
		profile.VPS.CreatedAt = ""
		if err := s.config.Save("./config.yaml"); err != nil {
			s.respondError(w, http.StatusInternalServerError, err)
			return
		}
	}

	s.respondSuccess(w, map[string]string{
		"message": "VPS destroyed successfully",
	})
}

// handleVPSRestart handles VPS restart
func (s *Server) handleVPSRestart(w http.ResponseWriter, r *http.Request) {
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

	if _, err := sshClient.ExecuteCommand("sudo reboot"); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": "VPS restart initiated",
	})
}

// handleVPSConnect returns SSH connection details
func (s *Server) handleVPSConnect(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	keyPath, _ := utils.ExpandPath(profile.SSH.KeyPath)

	s.respondSuccess(w, map[string]string{
		"host":     profile.VPS.PublicIP,
		"user":     profile.SSH.User,
		"key_path": keyPath,
		"command":  fmt.Sprintf("ssh -i %s %s@%s", keyPath, profile.SSH.User, profile.VPS.PublicIP),
	})
}

// handleHarden handles security hardening
func (s *Server) handleHarden(w http.ResponseWriter, r *http.Request) {
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

	hardenService := hardening.NewService(sshClient.GetClient())
	if err := hardenService.HardenVPS(context.Background()); err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}

	s.respondSuccess(w, map[string]string{
		"message": "Security hardening completed",
	})
}
