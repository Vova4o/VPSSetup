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

	// ListRegions lists available regions
	ListRegions(ctx context.Context) ([]Region, error)

	// ListSizes lists available instance sizes
	ListSizes(ctx context.Context) ([]Size, error)

	// ListImages lists available OS images
	ListImages(ctx context.Context) ([]Image, error)
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

// Region represents a datacenter region
type Region struct {
	Slug      string
	Name      string
	Available bool
}

// Size represents an instance size/plan
type Size struct {
	Slug         string
	Memory       int
	VCPUs        int
	Disk         int
	Transfer     float64
	PriceMonthly float64
	Available    bool
	Description  string
}

// Image represents an OS image
type Image struct {
	Slug         string
	Name         string
	Distribution string
	Public       bool
}
