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

			// If profile not specified, use default or ask user to select
			if profile == "" {
				if cfg.DefaultProfile != "" {
					profile = cfg.DefaultProfile
				} else if len(cfg.Profiles) == 1 {
					// If only one profile exists, use it
					for name := range cfg.Profiles {
						profile = name
						break
					}
				} else if len(cfg.Profiles) > 1 {
					// Multiple profiles, ask user to select
					profile, err = interactive.SelectProfile(cfg)
					if err != nil {
						return err
					}
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

	restartCmd = &cobra.Command{
		Use:   "restart",
		Short: "Restart the VPS",
		Long:  `Reboots the VPS instance.`,
		RunE:  runRestart,
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
		Use:   "logs",
		Short: "Fetch infrastructure logs from VPS",
		Long:  `Retrieves infrastructure logs from the VPS (nginx, system, fail2ban, ssh, firewall).`,
	}

	logsNginxCmd = &cobra.Command{
		Use:   "nginx [domain]",
		Short: "Fetch NGINX logs",
		Long:  `Retrieves NGINX access or error logs for a specific domain or the main nginx logs.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runLogsNginx,
	}

	logsSystemCmd = &cobra.Command{
		Use:   "system",
		Short: "Fetch system logs",
		Long:  `Retrieves system logs via journalctl.`,
		RunE:  runLogsSystem,
	}

	logsFail2banCmd = &cobra.Command{
		Use:   "fail2ban",
		Short: "Fetch fail2ban logs",
		Long:  `Retrieves fail2ban security logs.`,
		RunE:  runLogsFail2ban,
	}

	logsSSHCmd = &cobra.Command{
		Use:   "ssh",
		Short: "Fetch SSH authentication logs",
		Long:  `Retrieves SSH authentication logs from auth.log.`,
		RunE:  runLogsSSH,
	}

	logsFirewallCmd = &cobra.Command{
		Use:   "firewall",
		Short: "Fetch firewall logs",
		Long:  `Retrieves UFW firewall logs.`,
		RunE:  runLogsFirewall,
	}

	logsServiceCmd = &cobra.Command{
		Use:   "service <name>",
		Short: "Fetch logs for a systemd service",
		Long:  `Retrieves logs for a specific systemd service via journalctl.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runLogsService,
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

	dnsRemoveCmd = &cobra.Command{
		Use:   "remove [record-id]",
		Short: "Remove DNS record",
		Long:  `Removes a DNS record by ID.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDNSRemove,
	}

	dnsUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update DNS record",
		Long:  `Updates an existing DNS record.`,
		RunE:  runDNSUpdate,
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

	hardenCmd = &cobra.Command{
		Use:   "harden",
		Short: "Apply security hardening to VPS",
		Long:  `Performs security hardening on the VPS including system updates, fail2ban setup, SSH security, and automatic updates.`,
		RunE:  runHarden,
	}

	webCmd = &cobra.Command{
		Use:   "web",
		Short: "Start web API server",
		Long:  `Starts a local web server with REST API and UI for managing VPS deployments.`,
		RunE:  runWeb,
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Interactive setup wizard",
		Long:  `Run an interactive wizard to create a new configuration profile.`,
		RunE:  runInit,
	}

	nginxCmd = &cobra.Command{
		Use:   "nginx",
		Short: "Manage NGINX configuration",
		Long:  `Configure NGINX web server on your VPS with support for static sites, reverse proxy, and PHP applications.`,
	}

	nginxSetupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Setup NGINX configuration",
		Long:  `Interactive wizard to create and deploy NGINX configuration for your application.`,
		RunE:  runNginxSetup,
	}

	nginxTestCmd = &cobra.Command{
		Use:   "test",
		Short: "Test NGINX configuration",
		Long:  `Test the NGINX configuration for syntax errors.`,
		RunE:  runNginxTest,
	}

	nginxReloadCmd = &cobra.Command{
		Use:   "reload",
		Short: "Reload NGINX",
		Long:  `Reload NGINX to apply configuration changes.`,
		RunE:  runNginxReload,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./config.yaml", "config file path")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "profile to use (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "simulate actions without making changes")

	// Logs command flags (apply to all log subcommands)
	logsCmd.PersistentFlags().IntP("tail", "n", 100, "number of lines to show")
	logsCmd.PersistentFlags().StringP("grep", "g", "", "filter logs by pattern")
	logsNginxCmd.Flags().Bool("error", false, "show error logs instead of access logs")

	// Add SSL subcommands
	sslCmd.AddCommand(sslInstallCmd, sslRenewCmd, sslStatusCmd)

	// Add DNS subcommands
	dnsCmd.AddCommand(dnsCreateCmd, dnsListCmd, dnsUpdateCmd, dnsRemoveCmd)

	// Add NGINX subcommands
	nginxCmd.AddCommand(nginxSetupCmd, nginxTestCmd, nginxReloadCmd)

	// Add Logs subcommands
	logsCmd.AddCommand(logsNginxCmd, logsSystemCmd, logsFail2banCmd, logsSSHCmd, logsFirewallCmd, logsServiceCmd)

	// Add all commands to root
	rootCmd.AddCommand(
		initCmd,
		setupCmd,
		connectCmd,
		restartCmd,
		deployCmd,
		uploadCmd,
		logsCmd,
		sslCmd,
		dnsCmd,
		nginxCmd,
		statusCmd,
		hardenCmd,
		destroyCmd,
		webCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
