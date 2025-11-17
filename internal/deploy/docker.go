package deploy

import (
	"context"
	"fmt"
	"strings"

	"github.com/Vova4o/VPSSetup/internal/connection"
)

// DockerComposeDeployer handles Docker Compose deployments
type DockerComposeDeployer struct{}

// Deploy deploys using Docker Compose
func (d *DockerComposeDeployer) Deploy(ctx context.Context, opts DeployOptions) error {
	ssh := opts.SSH

	// 1. Install Docker if not present
	if err := d.installDocker(ssh); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// 2. Install Docker Compose if not present
	if err := d.installDockerCompose(ssh); err != nil {
		return fmt.Errorf("failed to install Docker Compose: %w", err)
	}

	// 3. Generate docker-compose.yml
	composeContent := d.generateDockerCompose(opts)

	// 4. Upload docker-compose.yml
	composeCmd := fmt.Sprintf("mkdir -p %s && cat > %s/docker-compose.yml << 'EOF'\n%s\nEOF",
		opts.RemotePath, opts.RemotePath, composeContent)
	if _, err := ssh.ExecuteCommand(composeCmd); err != nil {
		return fmt.Errorf("failed to upload docker-compose.yml: %w", err)
	}

	// 5. Generate .env file
	if len(opts.Env) > 0 {
		envContent := d.generateEnvFile(opts.Env)
		envCmd := fmt.Sprintf("cat > %s/.env << 'EOF'\n%s\nEOF", opts.RemotePath, envContent)
		if _, err := ssh.ExecuteCommand(envCmd); err != nil {
			return fmt.Errorf("failed to upload .env: %w", err)
		}
	}

	// 6. Pull images and start services
	deployCmd := fmt.Sprintf("cd %s && docker-compose pull && docker-compose up -d", opts.RemotePath)
	if _, err := ssh.ExecuteCommand(deployCmd); err != nil {
		return fmt.Errorf("failed to deploy: %w", err)
	}

	return nil
}

// Status checks deployment status
func (d *DockerComposeDeployer) Status(ctx context.Context) (*DeployStatus, error) {
	return &DeployStatus{Healthy: true}, nil
}

// Rollback rolls back to previous version
// TODO: Implement version tracking and rollback for docker-compose deployments
func (d *DockerComposeDeployer) Rollback(ctx context.Context) error {
	return fmt.Errorf("rollback not implemented yet")
}

// installDocker installs Docker on the VPS
func (d *DockerComposeDeployer) installDocker(ssh *connection.SSHClient) error {
	// Check if Docker is already installed
	_, err := ssh.ExecuteCommand("which docker")
	if err == nil {
		return nil // Already installed
	}

	// Install Docker
	commands := []string{
		"apt-get update",
		"apt-get install -y ca-certificates curl gnupg",
		"install -m 0755 -d /etc/apt/keyrings",
		"curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg",
		"chmod a+r /etc/apt/keyrings/docker.gpg",
		`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`,
		"apt-get update",
		"apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin",
	}

	for _, cmd := range commands {
		if _, err := ssh.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to execute: %s: %w", cmd, err)
		}
	}

	return nil
}

// installDockerCompose installs Docker Compose
func (d *DockerComposeDeployer) installDockerCompose(ssh *connection.SSHClient) error {
	// Check if already installed
	_, err := ssh.ExecuteCommand("which docker-compose")
	if err == nil {
		return nil
	}

	// Install Docker Compose plugin
	_, err = ssh.ExecuteCommand("apt-get install -y docker-compose-plugin")
	return err
}

// generateDockerCompose generates docker-compose.yml content
func (d *DockerComposeDeployer) generateDockerCompose(opts DeployOptions) string {
	var sb strings.Builder

	sb.WriteString("version: '3.8'\n\n")
	sb.WriteString("services:\n")

	// Add application service
	sb.WriteString("  app:\n")
	sb.WriteString("    build: .\n")
	sb.WriteString(fmt.Sprintf("    ports:\n      - \"%d:%d\"\n", opts.Port, opts.Port))

	// Add dependencies
	if len(opts.Services) > 0 {
		sb.WriteString("    depends_on:\n")
		for _, svc := range opts.Services {
			sb.WriteString(fmt.Sprintf("      - %s\n", svc.Name))
		}
	}

	// Add environment variables
	if len(opts.Env) > 0 {
		sb.WriteString("    environment:\n")
		for key := range opts.Env {
			sb.WriteString(fmt.Sprintf("      %s: ${%s}\n", key, key))
		}
	}

	sb.WriteString("    restart: unless-stopped\n")
	sb.WriteString("    networks:\n      - app-network\n\n")

	// Add additional services
	for _, svc := range opts.Services {
		sb.WriteString(d.generateServiceConfig(svc))
	}

	// Add networks
	sb.WriteString("networks:\n")
	sb.WriteString("  app-network:\n")
	sb.WriteString("    driver: bridge\n\n")

	// Add volumes
	if d.hasVolumes(opts.Services) {
		sb.WriteString("volumes:\n")
		for _, svc := range opts.Services {
			if len(svc.Volumes) > 0 || d.needsVolume(svc.Type) {
				sb.WriteString(fmt.Sprintf("  %s_data:\n", svc.Name))
			}
		}
	}

	return sb.String()
}

// generateServiceConfig generates config for a service
func (d *DockerComposeDeployer) generateServiceConfig(svc Service) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("  %s:\n", svc.Name))

	switch svc.Type {
	case "postgres":
		sb.WriteString(fmt.Sprintf("    image: postgres:%s\n", svc.Version))
		sb.WriteString("    environment:\n")
		sb.WriteString("      POSTGRES_DB: ${DB_NAME}\n")
		sb.WriteString("      POSTGRES_USER: ${DB_USER}\n")
		sb.WriteString("      POSTGRES_PASSWORD: ${DB_PASSWORD}\n")
		sb.WriteString("    volumes:\n")
		sb.WriteString(fmt.Sprintf("      - %s_data:/var/lib/postgresql/data\n", svc.Name))

	case "redis":
		sb.WriteString(fmt.Sprintf("    image: redis:%s\n", svc.Version))
		sb.WriteString("    volumes:\n")
		sb.WriteString(fmt.Sprintf("      - %s_data:/data\n", svc.Name))

	case "rabbitmq":
		sb.WriteString(fmt.Sprintf("    image: rabbitmq:%s-management\n", svc.Version))
		sb.WriteString("    environment:\n")
		sb.WriteString("      RABBITMQ_DEFAULT_USER: ${RABBITMQ_USER}\n")
		sb.WriteString("      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_PASSWORD}\n")
		sb.WriteString("    volumes:\n")
		sb.WriteString(fmt.Sprintf("      - %s_data:/var/lib/rabbitmq\n", svc.Name))
	}

	if svc.Port > 0 {
		sb.WriteString(fmt.Sprintf("    ports:\n      - \"%d:%d\"\n", svc.Port, svc.Port))
	}

	sb.WriteString("    restart: unless-stopped\n")
	sb.WriteString("    networks:\n      - app-network\n\n")

	return sb.String()
}

// generateEnvFile generates .env file content
func (d *DockerComposeDeployer) generateEnvFile(env map[string]string) string {
	var sb strings.Builder

	for key, value := range env {
		sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	return sb.String()
}

// hasVolumes checks if any service has volumes
func (d *DockerComposeDeployer) hasVolumes(services []Service) bool {
	for _, svc := range services {
		if len(svc.Volumes) > 0 || d.needsVolume(svc.Type) {
			return true
		}
	}
	return false
}

// needsVolume checks if service type needs a volume
func (d *DockerComposeDeployer) needsVolume(serviceType string) bool {
	return serviceType == "postgres" || serviceType == "redis" || serviceType == "rabbitmq"
}
