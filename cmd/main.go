package main

import (
	"fmt"
	"os"

	"github.com/Vova4o/VPSSetup/internal/config"
	"github.com/Vova4o/VPSSetup/internal/interactive"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	profile string
	verbose bool
	dryRun  bool
	cfg     *config.Config

	rootCmd = &cobra.Command{
		Use:   "vpssetup",
		Short: "VPS Setup Automation - Deploy projects to VPS with ease",
		Long: `A fast and reliable VPS setup automation tool for quick deployment 
of web projects with subdomain registration, NGINX configuration, and SSL certificate setup.

Sick and tired of VPS configuration? No more jumping between dashboards and SSH terminals!
This tool automates everything: VPS creation, DNS setup, NGINX config, and SSL certificates.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config loading for help, version, and init commands
			if cmd.Name() == "help" || cmd.Name() == "version" || cmd.Name() == "init" {
				return nil
			}

			var err error
			cfg, err = config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			// If profile not specified, ask user to select one interactively
			if profile == "" && len(cfg.Profiles) > 1 {
				profile, err = interactive.SelectProfile(cfg)
				if err != nil {
					return err
				}
			}

			if verbose {
				fmt.Printf("Using config file: %s\n", cfgFile)
				fmt.Printf("Using profile: %s\n", profile)
			}

			return nil
		},
	}

	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Setup and provision a new VPS instance",
		Long:  `Creates a new VPS instance, configures firewall, and performs initial security hardening.`,
		RunE:  runSetup,
	}

	connectCmd = &cobra.Command{
		Use:   "connect",
		Short: "Test SSH connection to VPS",
		Long:  `Establishes and validates SSH connection to the VPS instance.`,
		RunE:  runConnect,
	}

	deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Full deployment: setup, upload, configure, and start",
		Long: `Performs a complete deployment pipeline:
  1. Creates VPS instance
  2. Configures SSH connection
  3. Uploads project files
  4. Installs dependencies
  5. Configures NGINX
  6. Sets up SSL certificate
  7. Registers subdomain
  8. Starts the application`,
		RunE: runDeploy,
	}

	uploadCmd = &cobra.Command{
		Use:   "upload",
		Short: "Upload project files to VPS",
		Long:  `Uploads your project files to the VPS using SFTP/SCP.`,
		RunE:  runUpload,
	}

	logsCmd = &cobra.Command{
		Use:   "logs [service]",
		Short: "Fetch logs from VPS",
		Long:  `Retrieves application or system logs from the VPS.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runLogs,
	}

	sslCmd = &cobra.Command{
		Use:   "ssl",
		Short: "Manage SSL certificates",
		Long:  `Install, renew, or check status of SSL certificates.`,
	}

	sslInstallCmd = &cobra.Command{
		Use:   "install",
		Short: "Install SSL certificate",
		Long:  `Installs Let's Encrypt SSL certificate for the domain.`,
		RunE:  runSSLInstall,
	}

	sslRenewCmd = &cobra.Command{
		Use:   "renew",
		Short: "Renew SSL certificate",
		Long:  `Manually renews the SSL certificate.`,
		RunE:  runSSLRenew,
	}

	sslStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check SSL certificate status",
		Long:  `Shows SSL certificate information and expiration date.`,
		RunE:  runSSLStatus,
	}

	dnsCmd = &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS records",
		Long:  `Create, update, or list DNS records.`,
	}

	dnsCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create DNS record",
		Long:  `Creates a DNS record for the subdomain.`,
		RunE:  runDNSCreate,
	}

	dnsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List DNS records",
		Long:  `Lists all DNS records for the domain.`,
		RunE:  runDNSList,
	}

	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check VPS status",
		Long:  `Shows the status of your VPS instance and deployed application.`,
		RunE:  runStatus,
	}

	destroyCmd = &cobra.Command{
		Use:   "destroy",
		Short: "Destroy VPS instance",
		Long:  `Deletes the VPS instance. This action cannot be undone!`,
		RunE:  runDestroy,
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Interactive setup wizard",
		Long:  `Run an interactive wizard to create a new configuration profile.`,
		RunE:  runInit,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "profile to use (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")

	// Logs command flags
	logsCmd.Flags().IntP("tail", "n", 100, "number of lines to show")
	logsCmd.Flags().BoolP("follow", "f", false, "stream logs in real-time")

	// Add SSL subcommands
	sslCmd.AddCommand(sslInstallCmd, sslRenewCmd, sslStatusCmd)

	// Add DNS subcommands
	dnsCmd.AddCommand(dnsCreateCmd, dnsListCmd)

	// Add all commands to root
	rootCmd.AddCommand(
		initCmd,
		setupCmd,
		connectCmd,
		deployCmd,
		uploadCmd,
		logsCmd,
		sslCmd,
		dnsCmd,
		statusCmd,
		destroyCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
