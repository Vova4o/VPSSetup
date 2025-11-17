package hardening

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Service handles security hardening operations
type Service struct {
	sshClient *ssh.Client
}

// NewService creates a new hardening service
func NewService(sshClient *ssh.Client) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// HardenVPS performs basic security hardening
func (s *Service) HardenVPS(ctx context.Context) error {
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
			command: "sed -i 's/PermitRootLogin yes/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config",
		},
		{
			name:    "Disable password authentication",
			command: "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config",
		},
		{
			name:    "Restart SSH service",
			command: "systemctl restart sshd",
		},
		{
			name:    "Start fail2ban",
			command: "systemctl enable fail2ban && systemctl start fail2ban",
		},
	}

	for _, step := range steps {
		fmt.Printf("⚙️  %s...\n", step.name)

		if err := s.executeCommand(step.command); err != nil {
			return fmt.Errorf("failed at step '%s': %w", step.name, err)
		}

		fmt.Printf("✅ %s completed\n", step.name)
	}

	return nil
}

// executeCommand runs a command via SSH
func (s *Service) executeCommand(command string) error {
	session, err := s.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// GetHardeningScript returns a bash script for security hardening
// This can be used as UserData during droplet creation
func GetHardeningScript() string {
	script := `#!/bin/bash
set -e

# Update system
apt-get update -y
DEBIAN_FRONTEND=noninteractive apt-get upgrade -y

# Install security packages
DEBIAN_FRONTEND=noninteractive apt-get install -y fail2ban ufw unattended-upgrades apt-listchanges

# Configure automatic security updates
cat > /etc/apt/apt.conf.d/50unattended-upgrades << 'EEOF'
Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESMApps:${distro_codename}-apps-security";
    "${distro_id}ESM:${distro_codename}-infra-security";
};
Unattended-Upgrade::AutoFixInterruptedDpkg "true";
Unattended-Upgrade::MinimalSteps "true";
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
EEOF

# Enable automatic updates
cat > /etc/apt/apt.conf.d/20auto-upgrades << 'EEOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EEOF

# Configure SSH security
sed -i 's/#PermitRootLogin yes/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config
sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config
sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd

# Configure fail2ban
cat > /etc/fail2ban/jail.local << 'EEOF'
[DEFAULT]
bantime = 3600
findtime = 600
maxretry = 5

[sshd]
enabled = true
port = ssh
logpath = %(sshd_log)s
backend = %(sshd_backend)s
EEOF

systemctl enable fail2ban
systemctl start fail2ban

# Set timezone to UTC
timedatectl set-timezone UTC

# Disable unnecessary services
systemctl disable snapd 2>/dev/null || true

echo "Security hardening completed!"
`

	return strings.TrimSpace(script)
}
