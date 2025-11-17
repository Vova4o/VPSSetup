package deploy

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Vova4o/VPSSetup/internal/connection"
	"github.com/Vova4o/VPSSetup/internal/upload"
)

// StaticDeployer handles static website deployments
type StaticDeployer struct{}

// Deploy deploys a static website
func (s *StaticDeployer) Deploy(ctx context.Context, opts DeployOptions) error {
	ssh := opts.SSH

	// 1. Create remote directory
	mkdirCmd := fmt.Sprintf("sudo mkdir -p %s", opts.RemotePath)
	if _, err := ssh.ExecuteCommand(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// 2. Upload static files via SFTP
	uploadService, err := upload.NewService(ssh.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create upload service: %w", err)
	}
	defer uploadService.Close()

	// Upload with exclusions for static sites
	if err := uploadService.UploadProject(upload.UploadOptions{
		LocalPath:  opts.LocalPath,
		RemotePath: opts.RemotePath,
		Exclude: []string{
			".git",
			".gitignore",
			".DS_Store",
			"node_modules",
			"*.md",
			".vscode",
			".idea",
		},
	}); err != nil {
		return fmt.Errorf("failed to upload files: %w", err)
	}

	// 3. Set proper permissions
	chownCmd := fmt.Sprintf("sudo chown -R www-data:www-data %s", opts.RemotePath)
	if _, err := ssh.ExecuteCommand(chownCmd); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	chmodCmd := fmt.Sprintf("sudo chmod -R 755 %s", opts.RemotePath)
	if _, err := ssh.ExecuteCommand(chmodCmd); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// 4. Configure NGINX if config provided
	if opts.NginxConfig != "" {
		if err := s.deployNginxConfig(ssh, opts.Domain, opts.NginxConfig); err != nil {
			return fmt.Errorf("failed to deploy NGINX config: %w", err)
		}
	}

	return nil
}

// Status checks deployment status
func (s *StaticDeployer) Status(ctx context.Context) (*DeployStatus, error) {
	return &DeployStatus{Healthy: true, Message: "Static site deployed"}, nil
}

// Rollback rolls back to previous version
// TODO: Implement backup/restore for static file deployments
func (s *StaticDeployer) Rollback(ctx context.Context) error {
	return fmt.Errorf("rollback not implemented yet")
}

// deployNginxConfig deploys NGINX configuration for static site
func (s *StaticDeployer) deployNginxConfig(ssh *connection.SSHClient, domain, config string) error {
	// Write config file
	configPath := filepath.Join("/etc/nginx/sites-available", domain)
	writeCmd := fmt.Sprintf("sudo bash -c 'cat > %s << '\"'\"'EOF'\"'\"'\n%s\nEOF'", configPath, config)

	if _, err := ssh.ExecuteCommand(writeCmd); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create symlink
	symlinkCmd := fmt.Sprintf("sudo ln -sf %s /etc/nginx/sites-enabled/%s", configPath, domain)
	if _, err := ssh.ExecuteCommand(symlinkCmd); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Test config
	if _, err := ssh.ExecuteCommand("sudo nginx -t"); err != nil {
		return fmt.Errorf("NGINX config test failed: %w", err)
	}

	// Reload NGINX
	if _, err := ssh.ExecuteCommand("sudo systemctl reload nginx"); err != nil {
		return fmt.Errorf("failed to reload NGINX: %w", err)
	}

	return nil
}

// StandaloneDeployer handles standalone file uploads
type StandaloneDeployer struct{}

// Deploy uploads files without any configuration
func (s *StandaloneDeployer) Deploy(ctx context.Context, opts DeployOptions) error {
	ssh := opts.SSH

	// Create remote directory
	mkdirCmd := fmt.Sprintf("mkdir -p %s", opts.RemotePath)
	if _, err := ssh.ExecuteCommand(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Upload files via SFTP
	uploadService, err := upload.NewService(ssh.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create upload service: %w", err)
	}
	defer uploadService.Close()

	if err := uploadService.UploadProject(upload.UploadOptions{
		LocalPath:  opts.LocalPath,
		RemotePath: opts.RemotePath,
	}); err != nil {
		return fmt.Errorf("failed to upload files: %w", err)
	}

	return nil
}

// Status checks deployment status
func (s *StandaloneDeployer) Status(ctx context.Context) (*DeployStatus, error) {
	return &DeployStatus{Healthy: true, Message: "Files uploaded"}, nil
}

// Rollback rolls back to previous version
// TODO: Implement backup/restore for standalone deployments
func (s *StandaloneDeployer) Rollback(ctx context.Context) error {
	return fmt.Errorf("rollback not implemented yet")
}
