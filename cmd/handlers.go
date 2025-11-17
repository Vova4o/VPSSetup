package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Vova4o/VPSSetup/internal/config"
	"github.com/Vova4o/VPSSetup/internal/connection"
	"github.com/Vova4o/VPSSetup/internal/deploy"
	"github.com/Vova4o/VPSSetup/internal/dns"
	"github.com/Vova4o/VPSSetup/internal/interactive"
	"github.com/Vova4o/VPSSetup/internal/logs"
	"github.com/Vova4o/VPSSetup/internal/nginx"
	"github.com/Vova4o/VPSSetup/internal/setup"
	"github.com/Vova4o/VPSSetup/internal/ssl"
	"github.com/Vova4o/VPSSetup/internal/upload"
	"github.com/Vova4o/VPSSetup/pkg/provider"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// sshCommandExecutor adapts SSHClient to ssl.SSHExecutor interface
type sshCommandExecutor struct {
	client *connection.SSHClient
}

func (e *sshCommandExecutor) RunCommand(cmd string) (string, error) {
	return e.client.ExecuteCommand(cmd)
}

// expandPath expands ~ to home directory
func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(homeDir, path[2:]), nil
	}
	return path, nil
}

// extractSlug extracts the slug from a formatted string like "slug (description)"
func extractSlug(formatted string) string {
	// Find the position of the first space followed by '('
	if idx := strings.Index(formatted, " ("); idx != -1 {
		return formatted[:idx]
	}
	// If no description, return as is
	return formatted
}

// runInTerminal executes a command in the terminal with inherited stdin/stdout/stderr
func runInTerminal(command string) error {
	// Use shell to execute the command
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// runInit handles the init command (interactive wizard)
func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🧙 Welcome to VPS Setup Interactive Wizard!")

	// Run the setup wizard
	prof, err := interactive.SetupWizard()
	if err != nil {
		return err
	}

	// Ask for profile name
	profileName, err := interactive.AskInput("Enter a name for this profile:", "production")
	if err != nil {
		return err
	}

	// Create config structure
	configData := map[string]interface{}{
		"profiles": map[string]interface{}{
			profileName: prof,
		},
		"default_profile": profileName,
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Write to file
	configPath := "./config.yaml"
	if err := os.WriteFile(configPath, yamlData, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Println()
	interactive.Success(fmt.Sprintf("Configuration saved to %s", configPath))
	interactive.Info(fmt.Sprintf("You can now run: vpssetup deploy --profile %s", profileName))

	return nil
}

// runSetup handles the setup command
func runSetup(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Only DigitalOcean is supported for now
	if prof.VPS.Provider != "digitalocean" {
		return fmt.Errorf("only DigitalOcean provider is currently supported")
	}

	fmt.Printf("🚀 Creating VPS with profile: %s\n\n", profile)

	// Ask for VPS configuration interactively using the existing function
	if err := interactive.AskVPSConfiguration(prof); err != nil {
		return err
	}

	// Ask for VPS name
	vpsName, err := interactive.AskInput("\nEnter VPS name:", fmt.Sprintf("vps-%s", profile))
	if err != nil {
		return err
	}

	// Extract slugs from formatted strings (they're like "slug (description)")
	regionSlug := extractSlug(prof.VPS.Region)
	sizeSlug := extractSlug(prof.VPS.Size)

	fmt.Println()
	fmt.Printf("   Region: %s\n", prof.VPS.Region)
	fmt.Printf("   Size: %s\n", prof.VPS.Size)
	fmt.Printf("   Name: %s\n", vpsName)
	fmt.Println()

	// Confirm
	confirmed, err := interactive.ConfirmAction("Create VPS with these settings?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Setup cancelled")
		return nil
	}

	provider := provider.NewDigitalOceanProvider(prof.VPS.APIKey)

	if dryRun {
		fmt.Println("✓ Dry run complete - no VPS created")
		return nil
	}

	// Expand SSH key path
	sshKeyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	// Read SSH public key
	sshPublicKey, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return fmt.Errorf("failed to read SSH public key: %w", err)
	}

	// Create setup service
	setupService := setup.NewService(provider)

	// Setup configuration (use extracted slugs, not the formatted strings)
	setupConfig := setup.Config{
		Name:              vpsName,
		Region:            regionSlug,
		Size:              sizeSlug,
		Image:             "ubuntu-22-04-x64",
		SSHPublicKey:      string(sshPublicKey),
		SSHKeyPath:        sshKeyPath,
		SSHUser:           prof.SSH.User,
		RunInitialUpdates: true, // Always run updates on initial setup
		Firewall: setup.FirewallConfig{
			Enabled:    true,
			AllowSSH:   true,
			AllowHTTP:  true,
			AllowHTTPS: true,
		},
	}

	// Create VPS
	instance, err := setupService.SetupVPS(cmd.Context(), setupConfig)
	if err != nil {
		return fmt.Errorf("failed to setup VPS: %w", err)
	}

	// Save instance info to config
	fmt.Println()
	interactive.Info("💾 Saving instance information to config...")

	prof.VPS.InstanceID = instance.ID
	prof.VPS.PublicIP = instance.PublicIP
	prof.VPS.Name = instance.Name
	prof.VPS.Region = instance.Region
	prof.VPS.CreatedAt = instance.CreatedAt

	if err := cfg.Save(cfgFile); err != nil {
		interactive.Warning(fmt.Sprintf("Failed to save config: %v", err))
		interactive.Info("⚠️  Instance created but config not updated. Save these details manually:")
	} else {
		interactive.Success("Instance information saved to config")
	}

	fmt.Println()
	interactive.Info(fmt.Sprintf("Instance ID: %s", instance.ID))
	interactive.Info(fmt.Sprintf("Public IP: %s", instance.PublicIP))
	interactive.Info(fmt.Sprintf("Name: %s", instance.Name))
	interactive.Info("")
	interactive.Info("Next steps:")
	interactive.Info("  1. Connect: ./bin/vpssetup connect")
	interactive.Info("  2. Harden: ./bin/vpssetup harden")

	return nil
}

// runConnect handles the connect command
func runConnect(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Check if VPS info exists
	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found in config")
		interactive.Info("Run 'vpssetup setup' first to create a VPS")
		return fmt.Errorf("no VPS configured for profile '%s'", profile)
	}

	fmt.Printf("🔌 Connecting to VPS...\n")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   IP: %s\n", prof.VPS.PublicIP)
	fmt.Printf("   User: %s\n", prof.SSH.User)
	fmt.Printf("   Key: %s\n", prof.SSH.KeyPath)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - would connect to VPS")
		return nil
	}

	// Expand SSH key path
	sshKeyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	// Build SSH command
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no %s@%s",
		sshKeyPath,
		prof.SSH.User,
		prof.VPS.PublicIP,
	)

	interactive.Info("Connecting via SSH...")
	interactive.Info(fmt.Sprintf("Command: %s", sshCmd))
	fmt.Println()

	// Execute SSH connection
	interactive.Success("Opening SSH connection...")
	interactive.Info("(To exit, type 'exit' or press Ctrl+D)")
	fmt.Println()

	return runInTerminal(sshCmd)
}

// runRestart handles the restart command
func runRestart(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Check if VPS exists
	if prof.VPS.InstanceID == "" {
		interactive.Error("No VPS instance found in config")
		interactive.Info("Run 'vpssetup setup' first to create a VPS")
		return fmt.Errorf("no VPS configured for profile '%s'", profile)
	}

	fmt.Printf("🔄 Restarting VPS...\n")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   Instance: %s\n", prof.VPS.Name)
	fmt.Printf("   IP: %s\n", prof.VPS.PublicIP)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - would restart VPS")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Restart this VPS?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Restart cancelled")
		return nil
	}

	// Create provider
	provider := provider.NewDigitalOceanProvider(prof.VPS.APIKey)

	// Reboot the droplet
	spinner := interactive.ShowSpinner("Sending reboot command...")
	err = provider.RebootInstance(cmd.Context(), prof.VPS.InstanceID)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to restart VPS")
		return fmt.Errorf("failed to reboot instance: %w", err)
	}

	interactive.Success("Reboot command sent successfully")
	interactive.Info("")
	interactive.Info("The VPS is now rebooting. This may take 1-2 minutes.")
	interactive.Info("You can check status with: ./bin/vpssetup status")
	interactive.Info("Or reconnect with: ./bin/vpssetup connect")

	return nil
}

// runDeploy handles the deploy command
func runDeploy(cmd *cobra.Command, args []string) error {
	interactive.Warning("⚠️  The 'deploy' command is currently equivalent to 'setup'")
	interactive.Info("It only creates the VPS infrastructure. Application deployment is not yet implemented.")
	interactive.Info("")
	interactive.Info("For now, use these steps:")
	interactive.Info("  1. ./vpssetup setup      - Create VPS with firewall")
	interactive.Info("  2. ssh root@<ip>         - Connect manually")
	interactive.Info("  3. Upload and configure  - Manual deployment")
	interactive.Info("")

	confirmed, err := interactive.ConfirmAction("Continue with VPS creation only?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Just run setup
	return runSetup(cmd, args)
}

// runUpload handles the upload command
func runUpload(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Check if VPS exists
	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found in config")
		interactive.Info("Run 'vpssetup setup' first to create a VPS")
		return fmt.Errorf("no VPS configured for profile '%s'", profile)
	}

	fmt.Println("🚀 Deployment Wizard")
	fmt.Println()

	// Ask for deployment type
	deployTypes := []string{
		"static - Static HTML/CSS/JS website",
		"docker-compose - Full-stack app with Docker Compose (DB, Redis, etc.)",
		"standalone - Manual deployment (upload files only)",
	}

	deployTypeStr, err := interactive.AskSelect("Select deployment type:", deployTypes)
	if err != nil {
		return err
	}

	switch {
	case strings.HasPrefix(deployTypeStr, "static"):
		return runStaticDeploy(cmd, prof)
	case strings.HasPrefix(deployTypeStr, "docker-compose"):
		return runDockerComposeDeploy(cmd, prof)
	case strings.HasPrefix(deployTypeStr, "standalone"):
		return runStandaloneDeploy(cmd, prof)
	default:
		return fmt.Errorf("unknown deployment type")
	}
}

// runStaticDeploy handles static website deployment
func runStaticDeploy(cmd *cobra.Command, prof *config.Profile) error {
	// Get local path
	localPath, err := interactive.AskInput("Enter local project path:", "./")
	if err != nil {
		return err
	}

	// Get domain
	domain, err := interactive.AskInput("Enter domain name:", prof.Domain.Name)
	if err != nil {
		return err
	}

	// Get remote path
	remotePath, err := interactive.AskInput("Enter remote deployment path:",
		fmt.Sprintf("/var/www/%s/html", domain))
	if err != nil {
		return err
	}

	fmt.Printf("\n📦 Deploying static website\n")
	fmt.Printf("   Local:  %s\n", localPath)
	fmt.Printf("   Remote: %s\n", remotePath)
	fmt.Printf("   Domain: %s\n", domain)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - would deploy static site")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Deploy static website?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Deployment cancelled")
		return nil
	}

	// Connect to VPS
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Generate NGINX config for static site
	nginxConfig := generateStaticConfig(map[string]interface{}{
		"ServerName":  domain,
		"RootPath":    remotePath,
		"EnableSSL":   false,
		"MaxBodySize": "10M",
		"AccessLog":   fmt.Sprintf("/var/log/nginx/%s.access.log", domain),
		"ErrorLog":    fmt.Sprintf("/var/log/nginx/%s.error.log", domain),
	})

	// Deploy
	deployer := &deploy.StaticDeployer{}
	spinner = interactive.ShowSpinner("Deploying files...")

	err = deployer.Deploy(cmd.Context(), deploy.DeployOptions{
		Type:        deploy.DeploymentTypeStatic,
		LocalPath:   localPath,
		RemotePath:  remotePath,
		SSH:         sshClient,
		Domain:      domain,
		NginxConfig: nginxConfig,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error("Deployment failed")
		return err
	}

	interactive.Success("Static website deployed successfully! 🎉")
	fmt.Println()
	interactive.Info("Next steps:")
	interactive.Info(fmt.Sprintf("  1. Point your domain DNS to: %s", prof.VPS.PublicIP))
	interactive.Info("  2. Install SSL: vpssetup ssl install")
	interactive.Info(fmt.Sprintf("  3. Visit: http://%s", domain))

	return nil
}

// runDockerComposeDeploy handles Docker Compose deployment
func runDockerComposeDeploy(cmd *cobra.Command, prof *config.Profile) error {
	fmt.Println("🐳 Docker Compose Deployment")
	fmt.Println()

	// Ask for services
	services := []deploy.Service{}

	addPostgres, _ := interactive.ConfirmAction("Include PostgreSQL database?")
	if addPostgres {
		version, _ := interactive.AskInput("PostgreSQL version:", "15")
		services = append(services, deploy.Service{
			Name:    "postgres",
			Type:    "postgres",
			Version: version,
			Port:    5432,
		})
	}

	addRedis, _ := interactive.ConfirmAction("Include Redis cache?")
	if addRedis {
		version, _ := interactive.AskInput("Redis version:", "7")
		services = append(services, deploy.Service{
			Name:    "redis",
			Type:    "redis",
			Version: version,
			Port:    6379,
		})
	}

	addRabbitMQ, _ := interactive.ConfirmAction("Include RabbitMQ message broker?")
	if addRabbitMQ {
		version, _ := interactive.AskInput("RabbitMQ version:", "3.12")
		services = append(services, deploy.Service{
			Name:    "rabbitmq",
			Type:    "rabbitmq",
			Version: version,
			Port:    5672,
		})
	}

	// Get app port
	portStr, err := interactive.AskInput("Application port:", "8080")
	if err != nil {
		return err
	}
	port := 8080
	fmt.Sscanf(portStr, "%d", &port)

	// Get remote path
	remotePath, err := interactive.AskInput("Remote deployment path:", "/opt/app")
	if err != nil {
		return err
	}

	// Generate environment variables
	env := make(map[string]string)
	if addPostgres {
		env["DB_NAME"] = "myapp"
		env["DB_USER"] = "myapp"
		env["DB_PASSWORD"] = deploy.GeneratePassword()
		env["DATABASE_URL"] = "postgres://${DB_USER}:${DB_PASSWORD}@postgres:5432/${DB_NAME}"
	}
	if addRedis {
		env["REDIS_URL"] = "redis://redis:6379"
	}
	if addRabbitMQ {
		env["RABBITMQ_USER"] = "myapp"
		env["RABBITMQ_PASSWORD"] = deploy.GeneratePassword()
		env["RABBITMQ_URL"] = "amqp://${RABBITMQ_USER}:${RABBITMQ_PASSWORD}@rabbitmq:5672"
	}

	fmt.Printf("\n🐳 Docker Compose Configuration\n")
	fmt.Printf("   Services: %d\n", len(services)+1)
	for _, svc := range services {
		fmt.Printf("     - %s (%s:%s)\n", svc.Name, svc.Type, svc.Version)
	}
	fmt.Printf("   App Port: %d\n", port)
	fmt.Printf("   Path: %s\n", remotePath)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - would deploy with Docker Compose")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Deploy with Docker Compose?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Deployment cancelled")
		return nil
	}

	// Connect to VPS
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Deploy
	deployer := &deploy.DockerComposeDeployer{}
	spinner = interactive.ShowSpinner("Installing Docker and deploying services...")

	err = deployer.Deploy(cmd.Context(), deploy.DeployOptions{
		Type:       deploy.DeploymentTypeDockerCompose,
		RemotePath: remotePath,
		SSH:        sshClient,
		Port:       port,
		Services:   services,
		Env:        env,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error("Deployment failed")
		return err
	}

	interactive.Success("Docker Compose deployment completed! 🎉")
	fmt.Println()
	interactive.Info("Environment variables (.env file):")
	for key, value := range env {
		interactive.Info(fmt.Sprintf("  %s=%s", key, value))
	}
	fmt.Println()
	interactive.Info("Next steps:")
	interactive.Info(fmt.Sprintf("  1. SSH: vpssetup connect"))
	interactive.Info(fmt.Sprintf("  2. cd %s", remotePath))
	interactive.Info("  3. docker-compose ps - Check running containers")
	interactive.Info("  4. docker-compose logs -f - View logs")

	return nil
}

// runStandaloneDeploy handles standalone file upload
func runStandaloneDeploy(cmd *cobra.Command, prof *config.Profile) error {
	localPath, err := interactive.AskInput("Enter local project path:", "./")
	if err != nil {
		return err
	}

	remotePath, err := interactive.AskInput("Enter remote deployment path:", "/opt/app")
	if err != nil {
		return err
	}

	fmt.Printf("\n📦 Uploading files\n")
	fmt.Printf("   Local:  %s\n", localPath)
	fmt.Printf("   Remote: %s\n", remotePath)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - would upload files")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Upload files?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Upload cancelled")
		return nil
	}

	// Connect and upload
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	uploadService, err := upload.NewService(sshClient.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create upload service: %w", err)
	}
	defer uploadService.Close()

	spinner = interactive.ShowSpinner("Uploading files...")
	err = uploadService.UploadProject(upload.UploadOptions{
		LocalPath:  localPath,
		RemotePath: remotePath,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error("Upload failed")
		return err
	}

	interactive.Success("Files uploaded successfully! 🎉")
	interactive.Info("You can now SSH in and configure your application manually")

	return nil
}

// runLogsNginx handles the logs nginx command
func runLogsNginx(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	domain := ""
	if len(args) > 0 {
		domain = args[0]
	}

	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")
	showError, _ := cmd.Flags().GetBool("error")

	logType := "access"
	if showError {
		logType = "error"
	}

	fmt.Printf("📋 Fetching NGINX %s logs\n", logType)
	if domain != "" {
		fmt.Printf("   Domain: %s\n", domain)
	}
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	// Connect to VPS
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Import logs package
	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetNginxLogs(domain, logType, logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runLogsSystem handles the logs system command
func runLogsSystem(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")

	fmt.Printf("📋 Fetching system logs\n")
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetSystemLogs(logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runLogsFail2ban handles the logs fail2ban command
func runLogsFail2ban(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")

	fmt.Printf("📋 Fetching fail2ban logs\n")
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetFail2banLogs(logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runLogsSSH handles the logs ssh command
func runLogsSSH(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")

	fmt.Printf("📋 Fetching SSH authentication logs\n")
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetSSHLogs(logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runLogsFirewall handles the logs firewall command
func runLogsFirewall(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")

	fmt.Printf("📋 Fetching firewall logs\n")
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetFirewallLogs(logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runLogsService handles the logs service command
func runLogsService(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		return fmt.Errorf("no VPS configured")
	}

	serviceName := args[0]
	tail, _ := cmd.Flags().GetInt("tail")
	grepPattern, _ := cmd.Flags().GetString("grep")

	fmt.Printf("📋 Fetching logs for service: %s\n", serviceName)
	fmt.Printf("   Lines: %d\n", tail)
	if grepPattern != "" {
		fmt.Printf("   Filter: %s\n", grepPattern)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	logsService := logs.NewService(&sshCommandExecutor{client: sshClient})

	spinner = interactive.ShowSpinner("Fetching logs...")
	output, err := logsService.GetServiceLogs(serviceName, logs.LogOptions{
		Lines: tail,
		Grep:  grepPattern,
	})
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to fetch logs: %v", err))
		return err
	}

	if strings.TrimSpace(output) == "" {
		interactive.Info("No logs found")
		return nil
	}

	fmt.Println(output)
	return nil
}

// runSSLInstall handles the ssl install command
func runSSLInstall(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ensure domain is configured
	domain := prof.FullDomain()
	if domain == "" {
		return fmt.Errorf("domain not configured in profile")
	}

	// Ensure email is configured
	if prof.SSL.Email == "" {
		email, err := interactive.AskInput("Enter email for Let's Encrypt notifications:", "")
		if err != nil {
			return err
		}
		prof.SSL.Email = email

		// Save updated config
		if err := cfg.Save(cfgFile); err != nil {
			interactive.Warning("Failed to save email to config")
		}
	}

	fmt.Printf("🔒 Installing SSL certificate for: %s\n", domain)
	fmt.Printf("   Email: %s\n", prof.SSL.Email)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no SSL installed")
		return nil
	}

	// Confirm before proceeding
	confirmed, err := interactive.ConfirmAction("Install SSL certificate?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Expand SSH key path
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	// Connect to VPS
	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		interactive.Error("Failed to create SSH client")
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		interactive.Error("Failed to connect to VPS")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Create SSL service wrapper
	sslService := ssl.NewService(&sshCommandExecutor{client: sshClient})

	// Install SSL certificate
	interactive.Info("This may take a few minutes...")
	sslConfig := ssl.Config{
		Domain:    domain,
		Email:     prof.SSL.Email,
		AutoRenew: prof.SSL.AutoRenew,
	}

	if err := sslService.Install(sslConfig); err != nil {
		interactive.Error("Failed to install SSL certificate")
		return err
	}

	interactive.Success("SSL certificate installed successfully! 🎉")
	interactive.Info(fmt.Sprintf("Your site is now available at: https://%s", domain))

	return nil
}

// runSSLRenew handles the ssl renew command
func runSSLRenew(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	domain := prof.FullDomain()
	if domain == "" {
		return fmt.Errorf("domain not configured in profile")
	}

	fmt.Printf("🔄 Renewing SSL certificate for: %s\n", domain)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no SSL renewed")
		return nil
	}

	// Confirm before proceeding
	confirmed, err := interactive.ConfirmAction("Renew SSL certificate?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Expand SSH key path
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	// Connect to VPS
	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		interactive.Error("Failed to create SSH client")
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		interactive.Error("Failed to connect to VPS")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Create SSL service
	sslService := ssl.NewService(&sshCommandExecutor{client: sshClient})

	// Renew certificate
	if err := sslService.Renew(domain); err != nil {
		interactive.Error("Failed to renew SSL certificate")
		return err
	}

	interactive.Success("SSL certificate renewed successfully! 🎉")

	return nil
}

// runSSLStatus handles the ssl status command
func runSSLStatus(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	domain := prof.FullDomain()
	if domain == "" {
		return fmt.Errorf("domain not configured in profile")
	}

	fmt.Printf("📊 Checking SSL status for: %s\n", domain)
	fmt.Println()

	// Expand SSH key path
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand key path: %w", err)
	}

	// Connect to VPS
	spinner := interactive.ShowSpinner("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		interactive.Error("Failed to create SSH client")
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		interactive.Error("Failed to connect to VPS")
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer sshClient.Close()
	spinner.Stop()

	// Create SSL service
	sslService := ssl.NewService(&sshCommandExecutor{client: sshClient})

	// Get certificate status
	spinner = interactive.ShowSpinner("Checking certificate...")
	certInfo, err := sslService.Status(domain)
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to get SSL status: %v", err))
		interactive.Info("Certificate may not be installed yet. Run: vpssetup ssl install")
		return err
	}

	// Display certificate information
	interactive.Success("SSL Certificate Information")
	fmt.Printf("\n")
	fmt.Printf("  🌐 Domain:       %s\n", certInfo.Domain)
	fmt.Printf("  🏢 Issuer:       %s\n", certInfo.Issuer)
	fmt.Printf("  📅 Valid From:   %s\n", certInfo.ValidFrom)
	fmt.Printf("  📅 Valid Until:  %s\n", certInfo.ValidUntil)

	// Color code days left
	if certInfo.DaysLeft > 30 {
		fmt.Printf("  ⏰ Days Left:    🟢 %d days\n", certInfo.DaysLeft)
	} else if certInfo.DaysLeft > 7 {
		fmt.Printf("  ⏰ Days Left:    🟡 %d days (consider renewing soon)\n", certInfo.DaysLeft)
	} else {
		fmt.Printf("  ⏰ Days Left:    🔴 %d days (URGENT: renew now!)\n", certInfo.DaysLeft)
	}

	// Check auto-renewal status
	fmt.Printf("\n")
	autoRenew, err := sslService.CheckAutoRenewal()
	if err == nil {
		if autoRenew {
			fmt.Printf("  🔄 Auto-renewal: 🟢 Enabled\n")
		} else {
			fmt.Printf("  🔄 Auto-renewal: 🔴 Disabled\n")
		}
	}

	fmt.Printf("\n")

	// Warning if expiring soon
	if certInfo.DaysLeft <= 30 {
		interactive.Warning("Certificate is expiring soon!")
		interactive.Info("Run: vpssetup ssl renew")
	}

	return nil
}

// ensureDomainConfigured checks and prompts for domain if not set
func ensureDomainConfigured(prof *config.Profile) error {
	if prof.Domain.Name != "" {
		return nil
	}

	interactive.Warning("Domain not configured")
	interactive.Info("Let's set up your domain for DNS management")
	fmt.Println()

	// Ask for domain name
	domainName, err := interactive.AskInput("Enter your domain name (e.g., example.com):", "")
	if err != nil {
		return err
	}

	if domainName == "" {
		return fmt.Errorf("domain name is required")
	}

	// Update profile
	prof.Domain.Name = domainName

	// Save to config
	if err := cfg.Save(cfgFile); err != nil {
		interactive.Warning("Failed to save domain to config")
		interactive.Info("You can manually add it to config.yaml later")
	} else {
		interactive.Success("Domain saved to config")
	}

	return nil
}

// runDNSCreate handles the dns create command
func runDNSCreate(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ensure domain is configured
	if err := ensureDomainConfigured(prof); err != nil {
		return err
	}

	// Check if VPS exists
	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found")
		interactive.Info("Run 'vpssetup setup' first to create a VPS")
		return fmt.Errorf("no VPS configured for profile '%s'", profile)
	}

	// Ask for DNS record details
	recordName, err := interactive.AskInput("Record name (e.g., 'www' or '@' for root):", "@")
	if err != nil {
		return err
	}

	recordType, err := interactive.AskSelect("Record type:", []string{"A", "AAAA", "CNAME", "TXT", "MX"})
	if err != nil {
		return err
	}

	// Default value is VPS IP for A records
	defaultValue := prof.VPS.PublicIP
	if recordType != "A" {
		defaultValue = ""
	}

	recordValue, err := interactive.AskInput(fmt.Sprintf("Record value (for %s record):", recordType), defaultValue)
	if err != nil {
		return err
	}

	fmt.Printf("\n🌐 Creating DNS record\n")
	fmt.Printf("   Domain: %s\n", prof.Domain.Name)
	fmt.Printf("   Name: %s\n", recordName)
	fmt.Printf("   Type: %s\n", recordType)
	fmt.Printf("   Value: %s\n", recordValue)
	fmt.Printf("   Provider: %s\n", prof.Domain.DNSProvider)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no DNS record created")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Create this DNS record?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Create DNS service
	dnsService, err := dns.NewService(prof.Domain.DNSProvider, prof.Domain.DNSAPIKey, prof.Domain.Name)
	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to create DNS service: %v", err))
		return err
	}

	// Create DNS record
	spinner := interactive.ShowSpinner("Creating DNS record...")
	record := dns.Record{
		Name:  recordName,
		Type:  recordType,
		Value: recordValue,
		TTL:   3600,
	}

	err = dnsService.CreateRecord(cmd.Context(), record)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to create DNS record")
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	interactive.Success("DNS record created successfully! 🎉")
	interactive.Info("")
	interactive.Info("Note: DNS propagation may take a few minutes to several hours")
	interactive.Info(fmt.Sprintf("You can check with: dig %s.%s", recordName, prof.Domain.Name))

	return nil
}

// runDNSList handles the dns list command
func runDNSList(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ensure domain is configured
	if err := ensureDomainConfigured(prof); err != nil {
		return err
	}

	fmt.Printf("📋 Listing DNS records for: %s\n", prof.Domain.Name)
	fmt.Printf("   Provider: %s\n", prof.Domain.DNSProvider)
	fmt.Println()

	// Create DNS service
	dnsService, err := dns.NewService(prof.Domain.DNSProvider, prof.Domain.DNSAPIKey, prof.Domain.Name)
	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to create DNS service: %v", err))
		return err
	}

	// List DNS records
	spinner := interactive.ShowSpinner("Fetching DNS records...")
	records, err := dnsService.ListRecords(cmd.Context())
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to fetch DNS records")
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	if len(records) == 0 {
		interactive.Info("No DNS records found")
		return nil
	}

	interactive.Success(fmt.Sprintf("Found %d DNS record(s)", len(records)))
	fmt.Println()

	// Display records in a table format
	fmt.Printf("%-10s %-20s %-10s %-40s %-10s\n", "ID", "NAME", "TYPE", "VALUE", "TTL")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────")
	for _, record := range records {
		name := record.Name
		if name == "" || name == "@" {
			name = "@"
		}
		fmt.Printf("%-10s %-20s %-10s %-40s %-10d\n",
			record.ID,
			name,
			record.Type,
			record.Value,
			record.TTL)
	}

	return nil
}

// runDNSRemove handles the dns remove command
func runDNSRemove(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ensure domain is configured
	if err := ensureDomainConfigured(prof); err != nil {
		return err
	}

	fmt.Printf("🗑️  Removing DNS record from: %s\n", prof.Domain.Name)
	fmt.Printf("   Provider: %s\n", prof.Domain.DNSProvider)
	fmt.Println()

	// Create DNS service
	dnsService, err := dns.NewService(prof.Domain.DNSProvider, prof.Domain.DNSAPIKey, prof.Domain.Name)
	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to create DNS service: %v", err))
		return err
	}

	// Get record ID from args or list and ask
	var selectedRecord *dns.Record
	if len(args) > 0 {
		// Record ID provided, we'll need to fetch the record details
		// For now, we'll just use the ID
		selectedRecord = &dns.Record{ID: args[0]}
	} else {
		// List records first
		spinner := interactive.ShowSpinner("Fetching DNS records...")
		records, err := dnsService.ListRecords(cmd.Context())
		spinner.Stop()

		if err != nil {
			interactive.Error("Failed to fetch DNS records")
			return fmt.Errorf("failed to list DNS records: %w", err)
		}

		if len(records) == 0 {
			interactive.Info("No DNS records found")
			return nil
		}

		// Build options list and keep record references
		options := make([]string, len(records))
		recordMap := make(map[string]*dns.Record)
		for i, record := range records {
			name := record.Name
			if name == "" || name == "@" {
				name = "@"
			}
			optionStr := fmt.Sprintf("%s: %s %s -> %s", record.ID, name, record.Type, record.Value)
			options[i] = optionStr
			recordMap[optionStr] = &records[i]
		}

		selected, err := interactive.AskSelect("Select record to remove:", options)
		if err != nil {
			return err
		}

		// Get the full record from the map
		selectedRecord = recordMap[selected]
		if selectedRecord == nil {
			return fmt.Errorf("invalid selection")
		}
	}

	fmt.Printf("   Record ID: %s\n", selectedRecord.ID)
	if selectedRecord.Type != "" {
		fmt.Printf("   Record Type: %s\n", selectedRecord.Type)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no DNS record removed")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Remove this DNS record?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Delete DNS record
	spinner := interactive.ShowSpinner("Removing DNS record...")
	err = dnsService.DeleteRecord(cmd.Context(), *selectedRecord)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to remove DNS record")
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	interactive.Success("DNS record removed successfully! 🎉")
	return nil
}

// runDNSUpdate handles the dns update command
func runDNSUpdate(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ensure domain is configured
	if err := ensureDomainConfigured(prof); err != nil {
		return err
	}

	fmt.Printf("✏️  Updating DNS record for: %s\n", prof.Domain.Name)
	fmt.Printf("   Provider: %s\n", prof.Domain.DNSProvider)
	fmt.Println()

	// Create DNS service
	dnsService, err := dns.NewService(prof.Domain.DNSProvider, prof.Domain.DNSAPIKey, prof.Domain.Name)
	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to create DNS service: %v", err))
		return err
	}

	// List records first
	spinner := interactive.ShowSpinner("Fetching DNS records...")
	records, err := dnsService.ListRecords(cmd.Context())
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to fetch DNS records")
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	if len(records) == 0 {
		interactive.Info("No DNS records found")
		return nil
	}

	// Build options list and keep record references
	options := make([]string, len(records))
	recordMap := make(map[string]*dns.Record)
	for i, record := range records {
		name := record.Name
		if name == "" || name == "@" {
			name = "@"
		}
		optionStr := fmt.Sprintf("%s: %s %s -> %s", record.ID, name, record.Type, record.Value)
		options[i] = optionStr
		recordMap[optionStr] = &records[i]
	}

	selected, err := interactive.AskSelect("Select record to update:", options)
	if err != nil {
		return err
	}

	// Get the full record from the map
	selectedRecord := recordMap[selected]
	if selectedRecord == nil {
		return fmt.Errorf("invalid selection")
	}

	fmt.Printf("\n   Current Record:\n")
	fmt.Printf("   Type:  %s\n", selectedRecord.Type)
	fmt.Printf("   Name:  %s\n", selectedRecord.Name)
	fmt.Printf("   Value: %s\n", selectedRecord.Value)
	fmt.Println()

	// Ask what to update
	updateOptions := []string{
		"Update Name/Subdomain",
		"Update Value/IP Address",
		"Update Both",
	}
	updateChoice, err := interactive.AskSelect("What would you like to update?", updateOptions)
	if err != nil {
		return err
	}

	updatedRecord := *selectedRecord // Copy the record

	switch updateChoice {
	case "Update Name/Subdomain":
		newName, err := interactive.AskInput("Enter new name/subdomain:", selectedRecord.Name)
		if err != nil {
			return err
		}
		updatedRecord.Name = newName

	case "Update Value/IP Address":
		newValue, err := interactive.AskInput("Enter new value/IP:", selectedRecord.Value)
		if err != nil {
			return err
		}
		updatedRecord.Value = newValue

	case "Update Both":
		newName, err := interactive.AskInput("Enter new name/subdomain:", selectedRecord.Name)
		if err != nil {
			return err
		}
		updatedRecord.Name = newName

		newValue, err := interactive.AskInput("Enter new value/IP:", selectedRecord.Value)
		if err != nil {
			return err
		}
		updatedRecord.Value = newValue
	}

	fmt.Printf("\n   New Record:\n")
	fmt.Printf("   Type:  %s\n", updatedRecord.Type)
	fmt.Printf("   Name:  %s\n", updatedRecord.Name)
	fmt.Printf("   Value: %s\n", updatedRecord.Value)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no DNS record updated")
		return nil
	}

	confirmed, err := interactive.ConfirmAction("Update this DNS record?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Cancelled")
		return nil
	}

	// Convert to dns.Record format
	oldRecord := dns.Record{
		ID:    selectedRecord.ID,
		Type:  selectedRecord.Type,
		Name:  selectedRecord.Name,
		Value: selectedRecord.Value,
		TTL:   selectedRecord.TTL,
	}

	newRecord := dns.Record{
		Type:  updatedRecord.Type,
		Name:  updatedRecord.Name,
		Value: updatedRecord.Value,
		TTL:   updatedRecord.TTL,
	}

	// Update DNS record (delete + recreate)
	spinner = interactive.ShowSpinner("Updating DNS record...")
	err = dnsService.UpdateRecord(cmd.Context(), oldRecord, newRecord)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to update DNS record")
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	interactive.Success("DNS record updated successfully! 🎉")

	return nil
}

// runStatus handles the status command
func runStatus(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("📊 Checking VPS status...\n")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   Provider: %s\n\n", prof.VPS.Provider)

	// Check if instance exists
	if prof.VPS.InstanceID == "" {
		interactive.Warning("No VPS instance found in this profile")
		interactive.Info("Run 'vpssetup deploy' to create a VPS instance")
		return nil
	}

	// Create provider and get instance information
	ctx := context.Background()
	var instance *provider.Instance

	switch prof.VPS.Provider {
	case "digitalocean":
		vpsProvider := provider.NewDigitalOceanProvider(prof.VPS.APIKey)
		var err error
		instance, err = vpsProvider.GetInstance(ctx, prof.VPS.InstanceID)
		if err != nil {
			interactive.Error(fmt.Sprintf("Failed to get instance status: %v", err))
			return err
		}
	case "sweb":
		// Sweb doesn't have VPS API yet, show config info only
		interactive.Warning("Sweb VPS status check not available via API")
		interactive.Info("Showing configuration information only")
		instance = &provider.Instance{
			ID:        prof.VPS.InstanceID,
			Name:      prof.VPS.Name,
			PublicIP:  prof.VPS.PublicIP,
			Status:    "unknown",
			Region:    prof.VPS.Region,
			Size:      prof.VPS.Size,
			CreatedAt: prof.VPS.CreatedAt,
		}
	default:
		return fmt.Errorf("unsupported provider: %s", prof.VPS.Provider)
	}

	// Display status information
	interactive.Success("VPS Instance Information")
	fmt.Printf("\n")
	fmt.Printf("  🏷️  Name:       %s\n", instance.Name)
	fmt.Printf("  🆔 ID:         %s\n", instance.ID)
	fmt.Printf("  📍 Status:     %s\n", formatStatus(instance.Status))
	fmt.Printf("  🌐 Public IP:  %s\n", instance.PublicIP)
	if instance.PrivateIP != "" {
		fmt.Printf("  🔒 Private IP: %s\n", instance.PrivateIP)
	}
	fmt.Printf("  📍 Region:     %s\n", instance.Region)
	fmt.Printf("  💾 Size:       %s\n", instance.Size)
	if instance.CreatedAt != "" {
		fmt.Printf("  📅 Created:    %s\n", instance.CreatedAt)
	}

	// Display domain information if available
	if prof.Domain.Name != "" {
		fmt.Printf("\n")
		interactive.Success("Domain Configuration")
		fmt.Printf("\n")
		fmt.Printf("  🌍 Domain:     %s\n", prof.FullDomain())
		if prof.Domain.DNSProvider != "" {
			fmt.Printf("  📡 DNS:        %s\n", prof.Domain.DNSProvider)
		}
	}

	// Display project information if available
	if prof.Project.Path != "" {
		fmt.Printf("\n")
		interactive.Success("Project Configuration")
		fmt.Printf("\n")
		if prof.Project.Runtime != "" {
			fmt.Printf("  ⚙️  Runtime:    %s\n", prof.Project.Runtime)
		}
		if prof.Project.Port > 0 {
			fmt.Printf("  🔌 Port:       %d\n", prof.Project.Port)
		}
	}

	fmt.Printf("\n")
	return nil
}

// formatStatus formats the instance status with emoji
func formatStatus(status string) string {
	switch status {
	case "active", "running":
		return "🟢 " + status
	case "new", "pending":
		return "🟡 " + status
	case "off", "stopped":
		return "🔴 " + status
	default:
		return "⚪ " + status
	}
}

// runDestroy handles the destroy command
func runDestroy(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Check if VPS exists
	if prof.VPS.InstanceID == "" {
		interactive.Error("No VPS instance found in this profile")
		interactive.Info("Nothing to destroy")
		return nil
	}

	interactive.Warning("⚠️  DANGER: This will permanently destroy the VPS instance!")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   Provider: %s\n", prof.VPS.Provider)
	if prof.VPS.Name != "" {
		fmt.Printf("   Instance: %s\n", prof.VPS.Name)
	}
	fmt.Printf("   Instance ID: %s\n", prof.VPS.InstanceID)
	if prof.VPS.PublicIP != "" {
		fmt.Printf("   Public IP: %s\n", prof.VPS.PublicIP)
	}
	if prof.FullDomain() != "" {
		fmt.Printf("   Domain: %s\n", prof.FullDomain())
	}
	fmt.Println()
	interactive.Warning("⚠️  All data on this VPS will be PERMANENTLY LOST!")
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - no VPS destroyed")
		return nil
	}

	// Double confirmation for safety
	confirmed, err := interactive.ConfirmAction("Are you sure you want to destroy this VPS?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Destroy cancelled")
		return nil
	}

	// Ask user to type the profile name to confirm
	profileName, err := interactive.AskInput(fmt.Sprintf("Type '%s' to confirm:", profile), "")
	if err != nil {
		return err
	}
	if profileName != profile {
		interactive.Error("Profile name doesn't match. Destroy cancelled.")
		return nil
	}

	// Final confirmation
	finalConfirm, err := interactive.ConfirmAction("FINAL WARNING: Destroy VPS now?")
	if err != nil {
		return err
	}
	if !finalConfirm {
		interactive.Info("Destroy cancelled")
		return nil
	}

	// Create provider based on provider type
	var vpsProvider provider.VPSProvider
	switch prof.VPS.Provider {
	case "digitalocean":
		vpsProvider = provider.NewDigitalOceanProvider(prof.VPS.APIKey)
	case "sweb":
		interactive.Error("SWeb provider does not support VPS destruction via API")
		interactive.Info("Please delete the VPS manually from your SWeb control panel")
		return fmt.Errorf("sweb provider does not support VPS API operations")
	default:
		return fmt.Errorf("unsupported provider: %s", prof.VPS.Provider)
	}

	// Delete the instance
	spinner := interactive.ShowSpinner("Destroying VPS instance...")
	err = vpsProvider.DeleteInstance(cmd.Context(), prof.VPS.InstanceID)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to destroy VPS instance")
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	interactive.Success("VPS instance destroyed successfully! 💥")
	fmt.Println()
	interactive.Info("Instance data:")
	interactive.Info(fmt.Sprintf("  Name: %s", prof.VPS.Name))
	interactive.Info(fmt.Sprintf("  ID: %s", prof.VPS.InstanceID))
	interactive.Info(fmt.Sprintf("  IP: %s", prof.VPS.PublicIP))
	fmt.Println()

	// Ask if user wants to clear the config
	clearConfig, err := interactive.ConfirmAction("Clear VPS configuration from profile?")
	if err == nil && clearConfig {
		// Clear VPS info from profile
		prof.VPS.InstanceID = ""
		prof.VPS.PublicIP = ""
		prof.VPS.Name = ""
		prof.VPS.Region = ""
		prof.VPS.CreatedAt = ""

		if err := cfg.Save(cfgFile); err != nil {
			interactive.Warning(fmt.Sprintf("Failed to update config: %v", err))
			interactive.Info("You may want to manually remove the VPS info from config.yaml")
		} else {
			interactive.Success("Profile configuration cleared")
		}
	}

	return nil
}

// runHarden handles the harden command
func runHarden(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Check if VPS exists
	if prof.VPS.PublicIP == "" {
		interactive.Error("No VPS IP address found in config")
		interactive.Info("Run 'vpssetup setup' first to create a VPS")
		return fmt.Errorf("no VPS configured for profile '%s'", profile)
	}

	fmt.Printf("🔒 Applying security hardening to VPS\n")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   VPS IP: %s\n", prof.VPS.PublicIP)
	fmt.Println()

	if dryRun {
		fmt.Println("✓ Dry run complete - hardening steps:")
		fmt.Println("  1. Update system packages")
		fmt.Println("  2. Install security tools (fail2ban, ufw)")
		fmt.Println("  3. Configure automatic security updates")
		fmt.Println("  4. Harden SSH configuration")
		fmt.Println("  5. Setup fail2ban")
		return nil
	}

	// Display what will be done
	fmt.Println("This will:")
	fmt.Println("  ✓ Update all system packages")
	fmt.Println("  ✓ Install fail2ban, ufw, and unattended-upgrades")
	fmt.Println("  ✓ Configure automatic security updates")
	fmt.Println("  ✓ Disable root password login")
	fmt.Println("  ✓ Disable SSH password authentication")
	fmt.Println("  ✓ Configure fail2ban for SSH protection")
	fmt.Println()

	confirmed, err := interactive.ConfirmAction("Apply security hardening?")
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Hardening cancelled")
		return nil
	}

	// Expand SSH key path
	sshKeyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	// Create SSH client
	interactive.Info("Connecting to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, sshKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	defer sshClient.Close()

	// Connect
	err = sshClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	interactive.Success("Connected to VPS")
	fmt.Println()

	// Execute hardening steps
	steps := []struct {
		name    string
		command string
	}{
		{
			name:    "Update package lists",
			command: "apt-get update -y",
		},
		{
			name:    "Upgrade packages",
			command: "DEBIAN_FRONTEND=noninteractive apt-get upgrade -y",
		},
		{
			name:    "Install security packages",
			command: "DEBIAN_FRONTEND=noninteractive apt-get install -y fail2ban ufw unattended-upgrades",
		},
		{
			name:    "Configure automatic security updates",
			command: "dpkg-reconfigure -f noninteractive unattended-upgrades",
		},
		{
			name:    "Disable root password login",
			command: "sed -i 's/^#\\?PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config",
		},
		{
			name:    "Disable password authentication",
			command: "sed -i 's/^#\\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config",
		},
		{
			name:    "Restart SSH service",
			command: "systemctl restart sshd",
		},
		{
			name:    "Enable and start fail2ban",
			command: "systemctl enable fail2ban && systemctl start fail2ban",
		},
	}

	for i, step := range steps {
		spinner := interactive.ShowSpinner(fmt.Sprintf("[%d/%d] %s...", i+1, len(steps), step.name))

		_, err := sshClient.ExecuteCommand(step.command)
		spinner.Stop()

		if err != nil {
			interactive.Error(fmt.Sprintf("Failed: %s", step.name))
			return fmt.Errorf("hardening step failed '%s': %w", step.name, err)
		}

		interactive.Success(fmt.Sprintf("[%d/%d] %s", i+1, len(steps), step.name))
	}

	fmt.Println()
	interactive.Success("Security hardening completed successfully! 🎉")
	interactive.Info("")
	interactive.Info("Your VPS is now hardened with:")
	interactive.Info("  ✓ Latest security updates installed")
	interactive.Info("  ✓ fail2ban protecting SSH")
	interactive.Info("  ✓ Password authentication disabled")
	interactive.Info("  ✓ Root password login disabled")
	interactive.Info("  ✓ Automatic security updates enabled")

	return nil
}

// runNginxSetup sets up NGINX on the VPS
func runNginxSetup(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.InstanceID == "" {
		return fmt.Errorf("no VPS instance found. Please run 'vpssetup setup' first")
	}

	interactive.Info("🌐 NGINX Configuration Setup")
	fmt.Println()

	// Install NGINX if not already installed
	installNginx, err := interactive.ConfirmAction("Install/update NGINX on the VPS?")
	if err != nil {
		return err
	}
	if !installNginx {
		interactive.Info("Skipping NGINX installation")
	} else {
		// Expand SSH key path
		keyPath, err := expandPath(prof.SSH.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to expand SSH key path: %w", err)
		}

		// Connect to VPS
		spinner := interactive.ShowSpinner("Installing NGINX...")
		sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
		if err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to create SSH client: %w", err)
		}

		if err := sshClient.Connect(); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to connect to VPS: %w", err)
		}
		defer sshClient.Close()

		// Use nginx service
		nginxSvc := nginx.NewService(&sshCommandExecutor{client: sshClient})
		if err := nginxSvc.Install(); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to install nginx: %w", err)
		}
		spinner.Stop()
		interactive.Success("NGINX installed successfully")
	}

	// Ask for domain name
	domain, err := interactive.AskInput("Enter domain name (e.g., example.com)", prof.Domain.Name)
	if err != nil {
		return err
	}
	if domain == "" {
		return fmt.Errorf("domain name is required")
	}

	// Ask for www alias
	includeWWW, err := interactive.ConfirmAction("Include www subdomain? (e.g., www.example.com)")
	if err != nil {
		return err
	}

	// Ask for config type
	configTypes := []string{
		"static - Static HTML/CSS/JS site",
		"proxy - Reverse proxy for Node.js/Go/Python apps",
		"php - PHP application with PHP-FPM",
	}
	configTypeStr, err := interactive.AskSelect("Select application type", configTypes)
	if err != nil {
		return err
	}

	var configType string
	var proxyPort int

	switch {
	case strings.HasPrefix(configTypeStr, "static"):
		configType = "static"
	case strings.HasPrefix(configTypeStr, "proxy"):
		configType = "proxy"
		portStr, err := interactive.AskInput("Enter application port", "3000")
		if err != nil {
			return err
		}
		fmt.Sscanf(portStr, "%d", &proxyPort)
		if proxyPort == 0 {
			proxyPort = 3000
		}
	case strings.HasPrefix(configTypeStr, "php"):
		configType = "php"
	}

	// Ask for root path (for static and PHP)
	var rootPath string
	if configType == "static" || configType == "php" {
		rootPath, err = interactive.AskInput("Enter document root path", fmt.Sprintf("/var/www/%s/html", domain))
		if err != nil {
			return err
		}
	}

	// Ask for SSL
	enableSSL, err := interactive.ConfirmAction("Configure SSL/TLS? (You'll need certificates)")
	if err != nil {
		return err
	}

	// Generate and upload configuration
	spinner := interactive.ShowSpinner("Generating NGINX configuration...")

	// Import nginx package
	nginxCfg := createNginxConfig(domain, configType, proxyPort, rootPath, includeWWW, enableSSL)

	// Generate config content
	configContent, err := generateNginxConfig(nginxCfg)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to generate config: %w", err)
	}

	spinner.Stop()

	// Show preview
	fmt.Println()
	interactive.Info("Generated NGINX configuration:")
	fmt.Println("---")
	fmt.Println(configContent)
	fmt.Println("---")
	fmt.Println()

	deployConfirm, err := interactive.ConfirmAction("Deploy this configuration to the VPS?")
	if err != nil {
		return err
	}
	if !deployConfirm {
		interactive.Info("Configuration not deployed")
		return nil
	}

	// Deploy configuration
	// Expand SSH key path and connect
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner = interactive.ShowSpinner("Deploying configuration to VPS...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	nginxSvc := nginx.NewService(&sshCommandExecutor{client: sshClient})
	if err := nginxSvc.Deploy(domain, configContent, rootPath, configType); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to deploy nginx config: %w", err)
	}
	spinner.Stop()

	interactive.Success("NGINX configuration deployed successfully! 🎉")
	fmt.Println()
	interactive.Info("Next steps:")

	if configType == "static" || configType == "php" {
		interactive.Info(fmt.Sprintf("  1. Upload your files to: %s", rootPath))
	}
	if configType == "proxy" {
		interactive.Info(fmt.Sprintf("  1. Ensure your app is running on port %d", proxyPort))
	}
	if !enableSSL {
		interactive.Info("  2. Configure SSL with: vpssetup nginx ssl <domain>")
	}
	interactive.Info(fmt.Sprintf("  3. Point your domain DNS to: %s", prof.VPS.PublicIP))

	return nil
}

// Helper function to create nginx config structure
func createNginxConfig(domain, configType string, proxyPort int, rootPath string, includeWWW, enableSSL bool) map[string]interface{} {
	cfg := map[string]interface{}{
		"ServerName":  domain,
		"ConfigType":  configType,
		"EnableSSL":   enableSSL,
		"EnableGzip":  true,
		"MaxBodySize": "10M",
		"AccessLog":   fmt.Sprintf("/var/log/nginx/%s.access.log", domain),
		"ErrorLog":    fmt.Sprintf("/var/log/nginx/%s.error.log", domain),
	}

	if includeWWW {
		cfg["ServerAlias"] = []string{fmt.Sprintf("www.%s", domain)}
	}

	if configType == "proxy" {
		cfg["ProxyPort"] = proxyPort
	}

	if rootPath != "" {
		cfg["RootPath"] = rootPath
	} else {
		cfg["RootPath"] = fmt.Sprintf("/var/www/%s/html", domain)
	}

	if enableSSL {
		cfg["SSLCertPath"] = fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
		cfg["SSLKeyPath"] = fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)
	}

	return cfg
}

// Helper function to generate nginx config from template
func generateNginxConfig(cfg map[string]interface{}) (string, error) {
	// Import at runtime to avoid circular dependencies
	nginxPkg := "github.com/Vova4o/VPSSetup/pkg/nginx"
	_ = nginxPkg // This is a placeholder - actual implementation below

	configType := cfg["ConfigType"].(string)

	// Create a simple template-based config for now
	var template string

	switch configType {
	case "static":
		template = generateStaticConfig(cfg)
	case "proxy":
		template = generateProxyConfig(cfg)
	case "php":
		template = generatePHPConfig(cfg)
	default:
		return "", fmt.Errorf("unsupported config type: %s", configType)
	}

	return template, nil
}

func generateStaticConfig(cfg map[string]interface{}) string {
	domain := cfg["ServerName"].(string)
	rootPath := cfg["RootPath"].(string)
	accessLog := cfg["AccessLog"].(string)
	errorLog := cfg["ErrorLog"].(string)

	serverName := domain
	if aliases, ok := cfg["ServerAlias"].([]string); ok && len(aliases) > 0 {
		serverName = domain + " " + strings.Join(aliases, " ")
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    root %s;
    index index.html index.htm;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ =404;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, rootPath, accessLog, errorLog, cfg["MaxBodySize"].(string))

	if cfg["EnableSSL"].(bool) {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root %s;
    index index.html index.htm;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ =404;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, cfg["SSLCertPath"].(string), cfg["SSLKeyPath"].(string),
			rootPath, accessLog, errorLog, cfg["MaxBodySize"].(string))
	}

	return config
}

func generateProxyConfig(cfg map[string]interface{}) string {
	domain := cfg["ServerName"].(string)
	proxyPort := cfg["ProxyPort"].(int)
	accessLog := cfg["AccessLog"].(string)
	errorLog := cfg["ErrorLog"].(string)

	serverName := domain
	if aliases, ok := cfg["ServerAlias"].([]string); ok && len(aliases) > 0 {
		serverName = domain + " " + strings.Join(aliases, " ")
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_cache_bypass $http_upgrade;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
}`, serverName, accessLog, errorLog, cfg["MaxBodySize"].(string), proxyPort)

	if cfg["EnableSSL"].(bool) {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_cache_bypass $http_upgrade;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
}`, serverName, cfg["SSLCertPath"].(string), cfg["SSLKeyPath"].(string),
			accessLog, errorLog, cfg["MaxBodySize"].(string), proxyPort)
	}

	return config
}

func generatePHPConfig(cfg map[string]interface{}) string {
	domain := cfg["ServerName"].(string)
	rootPath := cfg["RootPath"].(string)
	accessLog := cfg["AccessLog"].(string)
	errorLog := cfg["ErrorLog"].(string)

	serverName := domain
	if aliases, ok := cfg["ServerAlias"].([]string); ok && len(aliases) > 0 {
		serverName = domain + " " + strings.Join(aliases, " ")
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    root %s;
    index index.php index.html index.htm;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
    
    location ~ /\.ht {
        deny all;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, rootPath, accessLog, errorLog, cfg["MaxBodySize"].(string))

	if cfg["EnableSSL"].(bool) {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root %s;
    index index.php index.html index.htm;
    
    access_log %s;
    error_log %s;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
    
    location ~ /\.ht {
        deny all;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, cfg["SSLCertPath"].(string), cfg["SSLKeyPath"].(string),
			rootPath, accessLog, errorLog, cfg["MaxBodySize"].(string))
	}

	return config
}

// deploy/install helper functions were replaced by internal/nginx.Service methods.

// runNginxTest tests the NGINX configuration
func runNginxTest(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.InstanceID == "" {
		return fmt.Errorf("no VPS instance found")
	}

	// Expand SSH key path
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Testing NGINX configuration...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	nginxSvc := nginx.NewService(&sshCommandExecutor{client: sshClient})
	if err := nginxSvc.TestConfig(); err != nil {
		spinner.Stop()
		interactive.Error("NGINX configuration test failed")
		return err
	}

	spinner.Stop()
	interactive.Success("NGINX configuration is valid!")
	return nil
}

// runNginxReload reloads NGINX
func runNginxReload(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.InstanceID == "" {
		return fmt.Errorf("no VPS instance found")
	}

	// Expand SSH key path
	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	spinner := interactive.ShowSpinner("Reloading NGINX...")
	sshClient, err := connection.NewSSHClient(prof.VPS.PublicIP, prof.SSH.User, keyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := sshClient.Connect(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to connect to VPS: %w", err)
	}
	defer sshClient.Close()

	nginxSvc := nginx.NewService(&sshCommandExecutor{client: sshClient})
	if err := nginxSvc.Reload(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	spinner.Stop()
	interactive.Success("NGINX reloaded successfully!")
	return nil
}
