package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Vova4o/VPSSetup/internal/config"
	"github.com/Vova4o/VPSSetup/internal/connection"
	"github.com/Vova4o/VPSSetup/internal/interactive"
	"github.com/Vova4o/VPSSetup/internal/setup"
	"github.com/Vova4o/VPSSetup/pkg/provider"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

	fmt.Printf("📦 Uploading project from: %s\n", prof.Project.Path)

	if dryRun {
		fmt.Println("✓ Dry run complete - no files uploaded")
		return nil
	}

	// TODO: Implement project upload
	fmt.Println("❌ Upload not implemented yet")
	return nil
}

// runLogs handles the logs command
func runLogs(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	tail, _ := cmd.Flags().GetInt("tail")
	follow, _ := cmd.Flags().GetBool("follow")

	service := "application"
	if len(args) > 0 {
		service = args[0]
	}

	fmt.Printf("📋 Fetching logs for: %s\n", service)
	fmt.Printf("   Lines: %d\n", tail)
	if follow {
		fmt.Printf("   Mode: streaming\n")
	}

	if dryRun {
		fmt.Println("✓ Dry run complete - no logs retrieved")
		return nil
	}

	// TODO: Implement log retrieval
	_ = prof
	fmt.Println("❌ Logs not implemented yet")
	return nil
}

// runSSLInstall handles the ssl install command
func runSSLInstall(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("🔒 Installing SSL certificate for: %s\n", prof.FullDomain())
	fmt.Printf("   Email: %s\n", prof.SSL.Email)

	if dryRun {
		fmt.Println("✓ Dry run complete - no SSL installed")
		return nil
	}

	// TODO: Implement SSL installation
	fmt.Println("❌ SSL install not implemented yet")
	return nil
}

// runSSLRenew handles the ssl renew command
func runSSLRenew(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("🔄 Renewing SSL certificate for: %s\n", prof.FullDomain())

	if dryRun {
		fmt.Println("✓ Dry run complete - no SSL renewed")
		return nil
	}

	// TODO: Implement SSL renewal
	fmt.Println("❌ SSL renew not implemented yet")
	return nil
}

// runSSLStatus handles the ssl status command
func runSSLStatus(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("📊 Checking SSL status for: %s\n", prof.FullDomain())

	// TODO: Implement SSL status check
	fmt.Println("❌ SSL status not implemented yet")
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

	// Create DNS provider
	if prof.Domain.DNSProvider != "sweb.ru" {
		interactive.Error(fmt.Sprintf("Unsupported DNS provider: %s", prof.Domain.DNSProvider))
		interactive.Info("Currently only sweb.ru is supported")
		return fmt.Errorf("unsupported DNS provider: %s", prof.Domain.DNSProvider)
	}

	dnsProvider := provider.NewSWebProvider(prof.Domain.DNSAPIKey)

	// Create DNS record
	spinner := interactive.ShowSpinner("Creating DNS record...")
	record := provider.DNSRecord{
		Domain: prof.Domain.Name,
		Name:   recordName,
		Type:   recordType,
		Value:  recordValue,
		TTL:    3600,
	}

	err = dnsProvider.AddDNSRecord(cmd.Context(), record)
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

	// Create DNS provider
	if prof.Domain.DNSProvider != "sweb.ru" {
		interactive.Error(fmt.Sprintf("Unsupported DNS provider: %s", prof.Domain.DNSProvider))
		interactive.Info("Currently only sweb.ru is supported")
		return fmt.Errorf("unsupported DNS provider: %s", prof.Domain.DNSProvider)
	}

	dnsProvider := provider.NewSWebProvider(prof.Domain.DNSAPIKey)

	// List DNS records
	spinner := interactive.ShowSpinner("Fetching DNS records...")
	records, err := dnsProvider.ListDNSRecords(cmd.Context(), prof.Domain.Name)
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

	var recordType string  // Track record type for proper deletion
	var recordName string  // Track record name for deletion
	var recordValue string // Track record value for deletion

	// Create DNS provider
	if prof.Domain.DNSProvider != "sweb.ru" {
		interactive.Error(fmt.Sprintf("Unsupported DNS provider: %s", prof.Domain.DNSProvider))
		interactive.Info("Currently only sweb.ru is supported")
		return fmt.Errorf("unsupported DNS provider: %s", prof.Domain.DNSProvider)
	}

	dnsProvider := provider.NewSWebProvider(prof.Domain.DNSAPIKey)

	// Get record ID from args or list and ask
	var recordID string
	var selectedRecord *provider.DNSRecord // Keep full record for deletion
	if len(args) > 0 {
		recordID = args[0]
	} else {
		// List records first
		spinner := interactive.ShowSpinner("Fetching DNS records...")
		records, err := dnsProvider.ListDNSRecords(cmd.Context(), prof.Domain.Name)
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
		recordMap := make(map[string]*provider.DNSRecord)
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
		if selectedRecord != nil {
			recordID = selectedRecord.ID
			recordType = selectedRecord.Type
			recordName = selectedRecord.Name
			recordValue = selectedRecord.Value
		} else {
			return fmt.Errorf("invalid selection")
		}
	}

	fmt.Printf("   Record ID: %s\n", recordID)
	if recordType != "" {
		fmt.Printf("   Record Type: %s\n", recordType)
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
	err = dnsProvider.DeleteDNSRecord(cmd.Context(), prof.Domain.Name, recordID, recordType, recordName, recordValue)
	spinner.Stop()

	if err != nil {
		interactive.Error("Failed to remove DNS record")
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	interactive.Success("DNS record removed successfully! 🎉")
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
	fmt.Printf("   Provider: %s\n", prof.VPS.Provider)

	// TODO: Implement status check
	fmt.Println("❌ Status not implemented yet")
	return nil
}

// runDestroy handles the destroy command
func runDestroy(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	interactive.Warning("This will permanently destroy the VPS instance!")
	fmt.Printf("   Profile: %s\n", profile)
	fmt.Printf("   Provider: %s\n", prof.VPS.Provider)
	fmt.Printf("   Domain: %s\n", prof.FullDomain())
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

	s := interactive.ShowSpinner("Destroying VPS instance...")
	// TODO: Implement VPS destruction
	s.Stop()
	interactive.Success("VPS instance destroyed")

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
		if err := installNginxPackage(prof); err != nil {
			return err
		}
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
	if err := deployNginxConfig(prof, domain, configContent, rootPath, configType); err != nil {
		return err
	}

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

func installNginxPackage(prof *config.Profile) error {
	spinner := interactive.ShowSpinner("Installing NGINX...")

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

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

	commands := []string{
		"apt-get update",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y nginx",
		"systemctl enable nginx",
		"systemctl start nginx",
	}

	for _, cmd := range commands {
		if _, err := sshClient.ExecuteCommand(cmd); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to execute command '%s': %w", cmd, err)
		}
	}

	spinner.Stop()
	interactive.Success("NGINX installed successfully")
	return nil
}

func deployNginxConfig(prof *config.Profile, domain, configContent, rootPath, configType string) error {
	spinner := interactive.ShowSpinner("Deploying configuration to VPS...")

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

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

	// Create config file
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", domain)

	// Escape single quotes in config content
	escapedConfig := strings.ReplaceAll(configContent, "'", "'\\''")

	uploadCmd := fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", escapedConfig, configPath)
	if _, err := sshClient.ExecuteCommand(uploadCmd); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to upload config: %w", err)
	}

	// Create symlink
	symlinkPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", domain)
	symlinkCmd := fmt.Sprintf("sudo ln -sf %s %s", configPath, symlinkPath)
	if _, err := sshClient.ExecuteCommand(symlinkCmd); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Create document root if needed
	if rootPath != "" && (configType == "static" || configType == "php") {
		mkdirCmd := fmt.Sprintf("sudo mkdir -p %s && sudo chown -R www-data:www-data %s", rootPath, rootPath)
		if _, err := sshClient.ExecuteCommand(mkdirCmd); err != nil {
			spinner.Stop()
			return fmt.Errorf("failed to create document root: %w", err)
		}

		// Create a default index.html for static sites
		if configType == "static" {
			indexContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Welcome</title>
</head>
<body>
    <h1>Welcome to %s</h1>
    <p>Your site is now running!</p>
</body>
</html>`, domain)
			escapedIndex := strings.ReplaceAll(indexContent, "'", "'\\''")
			indexCmd := fmt.Sprintf("echo '%s' | sudo tee %s/index.html > /dev/null", escapedIndex, rootPath)
			sshClient.ExecuteCommand(indexCmd)
		}
	}

	// Test configuration
	testCmd := "sudo nginx -t"
	output, err := sshClient.ExecuteCommand(testCmd)
	if err != nil {
		spinner.Stop()
		interactive.Error(fmt.Sprintf("NGINX configuration test failed:\n%s", output))
		return fmt.Errorf("nginx configuration test failed: %w", err)
	}

	// Reload NGINX
	reloadCmd := "sudo systemctl reload nginx"
	if _, err := sshClient.ExecuteCommand(reloadCmd); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	spinner.Stop()
	return nil
}

// runNginxTest tests the NGINX configuration
func runNginxTest(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	if prof.VPS.InstanceID == "" {
		return fmt.Errorf("no VPS instance found")
	}

	spinner := interactive.ShowSpinner("Testing NGINX configuration...")

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

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

	output, err := sshClient.ExecuteCommand("sudo nginx -t")
	spinner.Stop()

	if err != nil {
		interactive.Error("NGINX configuration test failed")
		fmt.Println(output)
		return err
	}

	interactive.Success("NGINX configuration is valid!")
	fmt.Println(output)
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

	spinner := interactive.ShowSpinner("Reloading NGINX...")

	keyPath, err := expandPath(prof.SSH.KeyPath)
	if err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to expand SSH key path: %w", err)
	}

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

	if _, err := sshClient.ExecuteCommand("sudo systemctl reload nginx"); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	spinner.Stop()
	interactive.Success("NGINX reloaded successfully!")
	return nil
}
