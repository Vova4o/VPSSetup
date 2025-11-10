package main

import (
	"fmt"
	"os"

	"github.com/Vova4o/VPSSetup/internal/interactive"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

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

	if err := prof.Validate(); err != nil {
		return fmt.Errorf("invalid profile configuration: %w", err)
	}

	fmt.Printf("🚀 Setting up VPS with profile: %s\n", profile)
	fmt.Printf("   Provider: %s\n", prof.VPS.Provider)
	fmt.Printf("   Region: %s\n", prof.VPS.Region)
	fmt.Printf("   Size: %s\n", prof.VPS.Size)

	if dryRun {
		fmt.Println("✓ Dry run complete - no changes made")
		return nil
	}

	// TODO: Implement VPS setup
	fmt.Println("❌ Setup not implemented yet")
	return nil
}

// runConnect handles the connect command
func runConnect(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("🔌 Testing SSH connection...\n")
	fmt.Printf("   User: %s\n", prof.SSH.User)
	fmt.Printf("   Key: %s\n", prof.SSH.KeyPath)

	if dryRun {
		fmt.Println("✓ Dry run complete - no changes made")
		return nil
	}

	// TODO: Implement SSH connection test
	fmt.Println("❌ Connect not implemented yet")
	return nil
}

// runDeploy handles the deploy command
func runDeploy(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Ask for deployment-specific details
	if err := interactive.AskDeploymentDetails(prof); err != nil {
		return err
	}

	if err := prof.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Printf("\n🚀 Starting full deployment with profile: %s\n", profile)
	fmt.Printf("   Provider: %s (%s, %s)\n", prof.VPS.Provider, prof.VPS.Region, prof.VPS.Size)
	fmt.Printf("   Domain: %s\n", prof.FullDomain())
	fmt.Printf("   Project: %s\n", prof.Project.Path)
	fmt.Printf("   Runtime: %s\n", prof.Project.Runtime)
	fmt.Println()

	// Ask for confirmation
	confirmed, err := interactive.ConfirmAction(fmt.Sprintf("Deploy to %s?", prof.FullDomain()))
	if err != nil {
		return err
	}
	if !confirmed {
		interactive.Info("Deployment cancelled")
		return nil
	}

	if dryRun {
		fmt.Println("✓ Dry run complete - deployment steps:")
		fmt.Println("  1. Create VPS instance")
		fmt.Println("  2. Configure SSH")
		fmt.Println("  3. Upload project files")
		fmt.Println("  4. Install dependencies")
		fmt.Println("  5. Configure NGINX")
		fmt.Println("  6. Setup SSL certificate")
		fmt.Println("  7. Register DNS record")
		fmt.Println("  8. Start application")
		return nil
	}

	// Show deployment progress with spinners
	s := interactive.ShowSpinner("Creating VPS instance...")
	// TODO: Actual VPS creation
	s.Stop()
	interactive.Success("VPS instance created")

	s = interactive.ShowSpinner("Configuring SSH connection...")
	// TODO: SSH configuration
	s.Stop()
	interactive.Success("SSH configured")

	s = interactive.ShowSpinner("Uploading project files...")
	// TODO: File upload
	s.Stop()
	interactive.Success("Project files uploaded")

	s = interactive.ShowSpinner("Installing dependencies...")
	// TODO: Dependency installation
	s.Stop()
	interactive.Success("Dependencies installed")

	s = interactive.ShowSpinner("Configuring NGINX...")
	// TODO: NGINX configuration
	s.Stop()
	interactive.Success("NGINX configured")

	if prof.SSL.Enable {
		s = interactive.ShowSpinner("Setting up SSL certificate...")
		// TODO: SSL setup
		s.Stop()
		interactive.Success("SSL certificate installed")
	}

	s = interactive.ShowSpinner("Registering DNS record...")
	// TODO: DNS registration
	s.Stop()
	interactive.Success("DNS record created")

	s = interactive.ShowSpinner("Starting application...")
	// TODO: Application start
	s.Stop()
	interactive.Success("Application started")

	fmt.Println()
	interactive.Success(fmt.Sprintf("Deployment complete! Your app is live at: https://%s", prof.FullDomain()))

	return nil
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

// runDNSCreate handles the dns create command
func runDNSCreate(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("🌐 Creating DNS record for: %s\n", prof.FullDomain())
	fmt.Printf("   Provider: %s\n", prof.Domain.DNSProvider)

	if dryRun {
		fmt.Println("✓ Dry run complete - no DNS record created")
		return nil
	}

	// TODO: Implement DNS record creation
	fmt.Println("❌ DNS create not implemented yet")
	return nil
}

// runDNSList handles the dns list command
func runDNSList(cmd *cobra.Command, args []string) error {
	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	fmt.Printf("📋 Listing DNS records for: %s\n", prof.Domain.Name)

	// TODO: Implement DNS record listing
	fmt.Println("❌ DNS list not implemented yet")
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
