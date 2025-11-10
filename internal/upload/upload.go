package upload

import (
	"fmt"
	"io"
)

// Service handles file upload operations
type Service struct {
	sshClient interface{}
}

// NewService creates a new upload service
func NewService(sshClient interface{}) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// UploadProject uploads project files to VPS
func (s *Service) UploadProject(localPath, remotePath string) error {
	fmt.Printf("Uploading project from %s to %s...\n", localPath, remotePath)
	
	// TODO: Implement project upload
	// 1. Create remote directory
	// 2. Use SFTP/SCP to transfer files
	// 3. Exclude unnecessary files (.git, node_modules, etc.)
	// 4. Show progress
	
	return fmt.Errorf("not implemented yet")
}

// UploadFile uploads a single file
func (s *Service) UploadFile(localPath, remotePath string) error {
	// TODO: Implement file upload
	return fmt.Errorf("not implemented yet")
}

// DownloadFile downloads a file from VPS
func (s *Service) DownloadFile(remotePath, localPath string) error {
	// TODO: Implement file download
	return fmt.Errorf("not implemented yet")
}

// UploadStream uploads from a reader
func (s *Service) UploadStream(reader io.Reader, remotePath string) error {
	// TODO: Implement stream upload
	return fmt.Errorf("not implemented yet")
}
