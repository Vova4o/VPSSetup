package provider

import (
	"context"
	"fmt"
)

// DigitalOceanProvider implements VPSProvider for DigitalOcean
type DigitalOceanProvider struct {
	APIKey string
}

// NewDigitalOceanProvider creates a new DigitalOcean provider
func NewDigitalOceanProvider(apiKey string) *DigitalOceanProvider {
	return &DigitalOceanProvider{
		APIKey: apiKey,
	}
}

func (d *DigitalOceanProvider) CreateInstance(ctx context.Context, config InstanceConfig) (*Instance, error) {
	// TODO: Implement DigitalOcean droplet creation
	return nil, fmt.Errorf("not implemented yet")
}

func (d *DigitalOceanProvider) GetInstance(ctx context.Context, instanceID string) (*Instance, error) {
	// TODO: Implement DigitalOcean droplet retrieval
	return nil, fmt.Errorf("not implemented yet")
}

func (d *DigitalOceanProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	// TODO: Implement DigitalOcean droplet deletion
	return fmt.Errorf("not implemented yet")
}

func (d *DigitalOceanProvider) ListInstances(ctx context.Context) ([]*Instance, error) {
	// TODO: Implement DigitalOcean droplet listing
	return nil, fmt.Errorf("not implemented yet")
}
