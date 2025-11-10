package provider

import "context"

// VPSProvider defines the interface for VPS provider operations
type VPSProvider interface {
	// CreateInstance creates a new VPS instance
	CreateInstance(ctx context.Context, config InstanceConfig) (*Instance, error)
	
	// GetInstance retrieves information about an instance
	GetInstance(ctx context.Context, instanceID string) (*Instance, error)
	
	// DeleteInstance deletes a VPS instance
	DeleteInstance(ctx context.Context, instanceID string) error
	
	// ListInstances lists all instances
	ListInstances(ctx context.Context) ([]*Instance, error)
}

// InstanceConfig holds configuration for creating a VPS instance
type InstanceConfig struct {
	Name     string
	Region   string
	Size     string
	Image    string
	SSHKeys  []string
	UserData string
	Tags     []string
}

// Instance represents a VPS instance
type Instance struct {
	ID        string
	Name      string
	PublicIP  string
	PrivateIP string
	Status    string
	Region    string
	Size      string
	CreatedAt string
}
