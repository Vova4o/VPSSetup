package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// SSHKeyRequest represents SSH key generation request
type SSHKeyRequest struct {
	KeyPath   string `json:"key_path"`
	Overwrite bool   `json:"overwrite"`
}

// handleGenerateSSHKey generates a new SSH key pair
func (s *Server) handleGenerateSSHKey(w http.ResponseWriter, r *http.Request) {
	var req SSHKeyRequest
	if err := parseJSON(r, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Use default path if not provided
	if req.KeyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to get home directory: %w", err))
			return
		}
		req.KeyPath = filepath.Join(homeDir, ".ssh", "vpssetup_rsa")
	}

	// Expand ~ to home directory
	if req.KeyPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to get home directory: %w", err))
			return
		}
		req.KeyPath = filepath.Join(homeDir, req.KeyPath[1:])
	}

	// Create .ssh directory if it doesn't exist
	sshDir := filepath.Dir(req.KeyPath)
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to create .ssh directory: %w", err))
		return
	}

	// Check if key already exists
	if _, err := os.Stat(req.KeyPath); err == nil {
		if !req.Overwrite {
			// Key exists, return its info without overwriting
			publicKeyPath := req.KeyPath + ".pub"
			publicKeyBytes, _ := os.ReadFile(publicKeyPath)

			s.respondSuccess(w, map[string]interface{}{
				"message":         "SSH key already exists",
				"private_key":     req.KeyPath,
				"public_key":      publicKeyPath,
				"public_key_data": string(publicKeyBytes),
				"already_exists":  true,
			})
			return
		}
		// Overwrite requested, delete existing keys
		os.Remove(req.KeyPath)
		os.Remove(req.KeyPath + ".pub")
	}

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to generate key: %w", err))
		return
	}

	// Encode private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write private key to file
	privateKeyFile, err := os.OpenFile(req.KeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to create private key file: %w", err))
		return
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to write private key: %w", err))
		return
	}

	// Generate public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to generate public key: %w", err))
		return
	}

	// Write public key to file
	publicKeyPath := req.KeyPath + ".pub"
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0o644); err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to write public key: %w", err))
		return
	}

	s.respondSuccess(w, map[string]interface{}{
		"message":         "SSH key pair generated successfully",
		"private_key":     req.KeyPath,
		"public_key":      publicKeyPath,
		"public_key_data": string(publicKeyBytes),
	})
}
