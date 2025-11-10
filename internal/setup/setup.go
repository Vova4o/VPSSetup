package setup

import (
	"context"
	"fmt"

	"github.com/Vova4o/VPSSetup/pkg/provider"
)

// Service handles VPS setup operations
type Service struct {
	provider provider.VPSProvider
}

// NewService creates a new setup service
func NewService(p provider.VPSProvider) *Service {
	return &Service{
		provider: p,
	}
}

// SetupVPS performs initial VPS setup
func (s *Service) SetupVPS(ctx context.Context, config Config) error {
	fmt.Println("Setting up VPS...")
	
	// TODO: Implement VPS setup logic
	// 1. Create VPS instance
	// 2. Wait for instance to be ready
	// 3. Configure firewall
	// 4. Install basic packages
	// 5. Setup security hardening
	
	return fmt.Errorf("not implemented yet")
}

// Config holds setup configuration
type Config struct {
	Name      string
	Region    string
	Size      string
	SSHKeyID  string
	Firewall  FirewallConfig
}

// FirewallConfig holds firewall rules
type FirewallConfig struct {
	AllowSSH   bool
	AllowHTTP  bool
	AllowHTTPS bool
	CustomPorts []int
}
