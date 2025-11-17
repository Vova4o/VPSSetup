package upload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Service handles file upload operations via SFTP
type Service struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// UploadOptions configures upload behavior
type UploadOptions struct {
	LocalPath   string
	RemotePath  string
	Exclude     []string // Patterns to exclude
	Permissions os.FileMode
}

// NewService creates a new upload service
func NewService(sshClient *ssh.Client) (*Service, error) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &Service{
		sshClient:  sshClient,
		sftpClient: sftpClient,
	}, nil
}

// Close closes the SFTP connection
func (s *Service) Close() error {
	if s.sftpClient != nil {
		return s.sftpClient.Close()
	}
	return nil
}

// UploadProject uploads a project directory with exclusions
func (s *Service) UploadProject(opts UploadOptions) error {
	// Default exclusions if none provided
	if len(opts.Exclude) == 0 {
		opts.Exclude = []string{
			".git",
			".gitignore",
			".DS_Store",
			"node_modules",
			".env",
			"__pycache__",
			"*.pyc",
			".vscode",
			".idea",
			"vendor",
			"tmp",
			"temp",
		}
	}

	// Ensure remote directory exists
	if err := s.sftpClient.MkdirAll(opts.RemotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Walk through local directory
	return filepath.Walk(opts.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(opts.LocalPath, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Skip excluded paths
		if s.shouldExclude(relPath, opts.Exclude) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Remote path
		remotePath := filepath.Join(opts.RemotePath, relPath)
		remotePath = filepath.ToSlash(remotePath) // Convert to Unix path

		if info.IsDir() {
			// Create directory
			return s.sftpClient.MkdirAll(remotePath)
		}

		// Upload file
		return s.uploadFile(path, remotePath, info.Mode())
	})
}

// UploadFile uploads a single file
func (s *Service) UploadFile(localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	return s.uploadFile(localPath, remotePath, info.Mode())
}

// DownloadFile downloads a file from the VPS
func (s *Service) DownloadFile(remotePath, localPath string) error {
	// Open remote file
	remoteFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy data
	_, err = io.Copy(localFile, remoteFile)
	return err
}

// uploadFile uploads a single file with permissions
func (s *Service) uploadFile(localPath, remotePath string, mode os.FileMode) error {
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	remoteDir = filepath.ToSlash(remoteDir)
	if err := s.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Create remote file
	remoteFile, err := s.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy data
	if _, err := io.Copy(remoteFile, localFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions
	if err := s.sftpClient.Chmod(remotePath, mode); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// shouldExclude checks if a path matches any exclusion pattern
func (s *Service) shouldExclude(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Simple glob matching
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Check if path contains pattern (for directories like node_modules)
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// GetFileList lists files in a remote directory
func (s *Service) GetFileList(remotePath string) ([]os.FileInfo, error) {
	return s.sftpClient.ReadDir(remotePath)
}
