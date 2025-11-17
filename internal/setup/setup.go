package setup

import (
	"context"
	"fmt"
	"time"

	"github.com/Vova4o/VPSSetup/internal/connection"
	"github.com/Vova4o/VPSSetup/internal/interactive"
	"github.com/Vova4o/VPSSetup/pkg/provider"
)

// Service handles VPS setup operations
type Service struct {
	provider *provider.DigitalOceanProvider
}

// NewService creates a new setup service
func NewService(p *provider.DigitalOceanProvider) *Service {
	return &Service{
		provider: p,
	}
}

// SetupVPS performs initial VPS setup
func (s *Service) SetupVPS(ctx context.Context, config Config) (*provider.Instance, error) {
	spinner := interactive.ShowSpinner("Creating VPS instance...")

	// Step 1: Upload SSH key if provided (or find existing one)
	var sshKeyIDs []string
	if config.SSHPublicKey != "" {
		spinner.Stop()

		// First, check if the key already exists
		interactive.Info("Checking for existing SSH key...")
		existingKeyID, err := s.provider.FindSSHKeyByPublicKey(ctx, config.SSHPublicKey)
		if err != nil {
			interactive.Warning(fmt.Sprintf("Could not check existing keys: %v", err))
		}

		if existingKeyID != "" {
			// Key already exists, use it
			sshKeyIDs = append(sshKeyIDs, existingKeyID)
			interactive.Success(fmt.Sprintf("Using existing SSH key (ID: %s)", existingKeyID))
		} else {
			// Upload new key
			interactive.Info("Uploading SSH key to DigitalOcean...")
			keyID, err := s.provider.UploadSSHKey(ctx, config.Name+"-key", config.SSHPublicKey)
			if err != nil {
				interactive.Error(fmt.Sprintf("Failed to upload SSH key: %v", err))
				return nil, fmt.Errorf("failed to upload SSH key: %w", err)
			}
			sshKeyIDs = append(sshKeyIDs, keyID)
			interactive.Success(fmt.Sprintf("SSH key uploaded (ID: %s)", keyID))
		}

		spinner = interactive.ShowSpinner("Creating VPS instance...")
	} else if config.SSHKeyID != "" {
		sshKeyIDs = append(sshKeyIDs, config.SSHKeyID)
	}

	// Step 2: Create VPS instance
	instanceConfig := provider.InstanceConfig{
		Name:    config.Name,
		Region:  config.Region,
		Size:    config.Size,
		Image:   config.Image,
		SSHKeys: sshKeyIDs,
		Tags:    []string{"vpssetup", "automated"},
	}

	instance, err := s.provider.CreateInstance(ctx, instanceConfig)
	spinner.Stop()

	if err != nil {
		interactive.Error(fmt.Sprintf("Failed to create VPS: %v", err))
		return nil, fmt.Errorf("failed to create VPS instance: %w", err)
	}

	interactive.Success(fmt.Sprintf("VPS instance created: %s (IP: %s)", instance.Name, instance.PublicIP))

	// Step 3: Configure firewall
	if config.Firewall.Enabled {
		spinner = interactive.ShowSpinner("Configuring firewall rules...")

		rules := s.buildFirewallRules(config.Firewall)
		firewallID, err := s.provider.CreateFirewall(ctx, config.Name+"-firewall", instance.ID, rules)
		spinner.Stop()

		if err != nil {
			interactive.Warning(fmt.Sprintf("Failed to create firewall: %v", err))
			// Don't fail the entire setup if firewall creation fails
		} else {
			interactive.Success(fmt.Sprintf("Firewall configured (ID: %s)", firewallID))
		}
	}

	// Step 4: Wait for SSH to be available and run system updates
	if config.RunInitialUpdates {
		err = s.runInitialSetup(ctx, instance.PublicIP, config.SSHKeyPath, config.SSHUser)
		if err != nil {
			interactive.Warning(fmt.Sprintf("Initial updates failed: %v", err))
			interactive.Info("You may need to run updates manually later")
		}
	} else {
		interactive.Info("Waiting for SSH to become available...")
		time.Sleep(15 * time.Second) // Give the server time to boot
	}

	interactive.Success("VPS setup completed!")
	interactive.Info(fmt.Sprintf("  Name:      %s", instance.Name))
	interactive.Info(fmt.Sprintf("  Public IP: %s", instance.PublicIP))
	interactive.Info(fmt.Sprintf("  Region:    %s", instance.Region))
	interactive.Info(fmt.Sprintf("  Size:      %s", instance.Size))
	interactive.Info("")
	interactive.Info("Next steps:")
	interactive.Info(fmt.Sprintf("  1. Connect via SSH: ssh root@%s", instance.PublicIP))
	interactive.Info("  2. Run security hardening: ./vpssetup harden")
	interactive.Info("  3. Deploy your project: ./vpssetup deploy")

	return instance, nil
}

// runInitialSetup connects to VPS and runs system updates
func (s *Service) runInitialSetup(ctx context.Context, host, keyPath, user string) error {
	interactive.Info("Waiting for SSH to become available...")

	// Create SSH client
	sshClient, err := connection.NewSSHClient(host, user, keyPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}
	defer sshClient.Close()

	// Wait for SSH to be ready (5 minute timeout)
	spinner := interactive.ShowSpinner("Waiting for SSH connection...")
	err = sshClient.WaitForReady(5 * time.Minute)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("SSH connection timeout: %w", err)
	}

	interactive.Success("SSH connection established")

	// Wait for apt lock to be released (automatic updates may be running)
	spinner = interactive.ShowSpinner("Waiting for system to be ready...")

	waitForAptCmd := `
# Wait up to 5 minutes for apt lock to be released
for i in {1..60}; do
  if ! fuser /var/lib/apt/lists/lock >/dev/null 2>&1 && \
     ! fuser /var/lib/dpkg/lock >/dev/null 2>&1 && \
     ! fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1; then
    exit 0
  fi
  sleep 5
done
exit 1
`

	_, err = sshClient.ExecuteCommand(waitForAptCmd)
	spinner.Stop()

	if err != nil {
		interactive.Warning("Could not wait for apt lock, continuing anyway...")
	} else {
		interactive.Success("System is ready")
	}

	// Run system updates
	spinner = interactive.ShowSpinner("Running system updates (this may take a few minutes)...")

	updateCmd := `
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get autoremove -y -qq
apt-get autoclean -y -qq
`

	_, err = sshClient.ExecuteCommand(updateCmd)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("system update failed: %w", err)
	}

	interactive.Success("System updates completed")
	return nil
}

// buildFirewallRules constructs firewall rules from config
func (s *Service) buildFirewallRules(config FirewallConfig) []provider.FirewallRule {
	rules := []provider.FirewallRule{}

	if config.AllowSSH {
		rules = append(rules, provider.FirewallRule{
			Protocol:  "tcp",
			PortRange: "22",
			Sources:   []string{"0.0.0.0/0", "::/0"},
		})
	}

	if config.AllowHTTP {
		rules = append(rules, provider.FirewallRule{
			Protocol:  "tcp",
			PortRange: "80",
			Sources:   []string{"0.0.0.0/0", "::/0"},
		})
	}

	if config.AllowHTTPS {
		rules = append(rules, provider.FirewallRule{
			Protocol:  "tcp",
			PortRange: "443",
			Sources:   []string{"0.0.0.0/0", "::/0"},
		})
	}

	// Add custom ports
	for _, port := range config.CustomPorts {
		rules = append(rules, provider.FirewallRule{
			Protocol:  "tcp",
			PortRange: fmt.Sprintf("%d", port),
			Sources:   []string{"0.0.0.0/0", "::/0"},
		})
	}

	return rules
}

// Config holds setup configuration
type Config struct {
	Name              string
	Region            string
	Size              string
	Image             string
	SSHKeyID          string
	SSHPublicKey      string
	SSHKeyPath        string
	SSHUser           string
	RunInitialUpdates bool
	Firewall          FirewallConfig
}

// FirewallConfig holds firewall rules
type FirewallConfig struct {
	Enabled     bool
	AllowSSH    bool
	AllowHTTP   bool
	AllowHTTPS  bool
	CustomPorts []int
}
