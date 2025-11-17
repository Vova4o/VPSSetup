package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// DigitalOceanProvider implements VPSProvider for DigitalOcean
type DigitalOceanProvider struct {
	APIKey string
	client *godo.Client
}

// TokenSource implements oauth2.TokenSource
type TokenSource struct {
	AccessToken string
}

// Token returns an oauth2.Token
func (t *TokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: t.AccessToken,
	}, nil
}

// NewDigitalOceanProvider creates a new DigitalOcean provider
func NewDigitalOceanProvider(apiKey string) *DigitalOceanProvider {
	tokenSource := &TokenSource{
		AccessToken: apiKey,
	}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client := godo.NewClient(oauthClient)

	return &DigitalOceanProvider{
		APIKey: apiKey,
		client: client,
	}
}

// CreateInstance creates a new DigitalOcean droplet
func (d *DigitalOceanProvider) CreateInstance(ctx context.Context, config InstanceConfig) (*Instance, error) {
	// Convert string SSH keys to SSH key IDs
	sshKeys := make([]godo.DropletCreateSSHKey, 0, len(config.SSHKeys))
	for _, keyID := range config.SSHKeys {
		// Try to parse as int (ID), otherwise use as fingerprint
		if id, err := strconv.Atoi(keyID); err == nil {
			sshKeys = append(sshKeys, godo.DropletCreateSSHKey{ID: id})
		} else {
			sshKeys = append(sshKeys, godo.DropletCreateSSHKey{Fingerprint: keyID})
		}
	}

	// Default image if not specified
	image := config.Image
	if image == "" {
		image = "ubuntu-22-04-x64"
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   config.Name,
		Region: config.Region,
		Size:   config.Size,
		Image: godo.DropletCreateImage{
			Slug: image,
		},
		SSHKeys:  sshKeys,
		UserData: config.UserData,
		Tags:     config.Tags,
		IPv6:     true,
	}

	droplet, _, err := d.client.Droplets.Create(ctx, createRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create droplet: %w", err)
	}

	// Wait for droplet to be active and get IP address
	droplet, err = d.waitForDropletActive(ctx, droplet.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for droplet: %w", err)
	}

	return d.convertDropletToInstance(droplet), nil
}

// GetInstance retrieves information about a DigitalOcean droplet
func (d *DigitalOceanProvider) GetInstance(ctx context.Context, instanceID string) (*Instance, error) {
	id, err := strconv.Atoi(instanceID)
	if err != nil {
		return nil, fmt.Errorf("invalid instance ID: %w", err)
	}

	droplet, _, err := d.client.Droplets.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get droplet: %w", err)
	}

	return d.convertDropletToInstance(droplet), nil
}

// DeleteInstance deletes a DigitalOcean droplet
func (d *DigitalOceanProvider) DeleteInstance(ctx context.Context, instanceID string) error {
	id, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %w", err)
	}

	_, err = d.client.Droplets.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete droplet: %w", err)
	}

	return nil
}

// RebootInstance reboots a DigitalOcean droplet
func (d *DigitalOceanProvider) RebootInstance(ctx context.Context, instanceID string) error {
	id, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %w", err)
	}

	_, _, err = d.client.DropletActions.Reboot(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to reboot droplet: %w", err)
	}

	return nil
}

// ListInstances lists all DigitalOcean droplets
func (d *DigitalOceanProvider) ListInstances(ctx context.Context) ([]*Instance, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200,
	}

	var allDroplets []godo.Droplet
	for {
		droplets, resp, err := d.client.Droplets.List(ctx, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list droplets: %w", err)
		}

		allDroplets = append(allDroplets, droplets...)

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("failed to get current page: %w", err)
		}
		opt.Page = page + 1
	}

	instances := make([]*Instance, 0, len(allDroplets))
	for _, droplet := range allDroplets {
		instances = append(instances, d.convertDropletToInstance(&droplet))
	}

	return instances, nil
}

// UploadSSHKey uploads an SSH key to DigitalOcean
func (d *DigitalOceanProvider) UploadSSHKey(ctx context.Context, name, publicKey string) (string, error) {
	createRequest := &godo.KeyCreateRequest{
		Name:      name,
		PublicKey: publicKey,
	}

	key, _, err := d.client.Keys.Create(ctx, createRequest)
	if err != nil {
		return "", fmt.Errorf("failed to upload SSH key: %w", err)
	}

	return strconv.Itoa(key.ID), nil
}

// FindSSHKeyByPublicKey finds an SSH key by its public key content
func (d *DigitalOceanProvider) FindSSHKeyByPublicKey(ctx context.Context, publicKey string) (string, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 200,
	}

	for {
		keys, resp, err := d.client.Keys.List(ctx, opt)
		if err != nil {
			return "", fmt.Errorf("failed to list SSH keys: %w", err)
		}

		for _, key := range keys {
			if key.PublicKey == publicKey {
				return strconv.Itoa(key.ID), nil
			}
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return "", fmt.Errorf("failed to get current page: %w", err)
		}
		opt.Page = page + 1
	}

	return "", nil // Not found
}

// GetSSHKeyFingerprint gets the fingerprint of an SSH key by ID
func (d *DigitalOceanProvider) GetSSHKeyFingerprint(ctx context.Context, keyID string) (string, error) {
	id, err := strconv.Atoi(keyID)
	if err != nil {
		return "", fmt.Errorf("invalid key ID: %w", err)
	}

	key, _, err := d.client.Keys.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get SSH key: %w", err)
	}

	return key.Fingerprint, nil
}

// convertDropletToInstance converts a godo.Droplet to Instance
func (d *DigitalOceanProvider) convertDropletToInstance(droplet *godo.Droplet) *Instance {
	publicIP, _ := droplet.PublicIPv4()
	privateIP, _ := droplet.PrivateIPv4()

	return &Instance{
		ID:        strconv.Itoa(droplet.ID),
		Name:      droplet.Name,
		PublicIP:  publicIP,
		PrivateIP: privateIP,
		Status:    droplet.Status,
		Region:    droplet.Region.Slug,
		Size:      droplet.Size.Slug,
		CreatedAt: droplet.Created,
	}
}

// waitForDropletActive waits for a droplet to become active
func (d *DigitalOceanProvider) waitForDropletActive(ctx context.Context, dropletID int) (*godo.Droplet, error) {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for droplet to become active")
		case <-ticker.C:
			droplet, _, err := d.client.Droplets.Get(ctx, dropletID)
			if err != nil {
				return nil, err
			}

			if droplet.Status == "active" {
				// Additional wait for network to be fully ready
				time.Sleep(10 * time.Second)
				return droplet, nil
			}
		}
	}
}

// ListRegions lists all available DigitalOcean regions
func (d *DigitalOceanProvider) ListRegions(ctx context.Context) ([]Region, error) {
	regions, _, err := d.client.Regions.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list regions: %w", err)
	}

	result := make([]Region, 0, len(regions))
	for _, r := range regions {
		result = append(result, Region{
			Slug:      r.Slug,
			Name:      r.Name,
			Available: r.Available,
		})
	}

	return result, nil
}

// ListSizes lists all available DigitalOcean droplet sizes
func (d *DigitalOceanProvider) ListSizes(ctx context.Context) ([]Size, error) {
	sizes, _, err := d.client.Sizes.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list sizes: %w", err)
	}

	result := make([]Size, 0, len(sizes))
	for _, s := range sizes {
		description := fmt.Sprintf("%s - %dGB RAM, %d vCPU, %dGB SSD ($%.2f/mo)",
			s.Slug, s.Memory/1024, s.Vcpus, s.Disk, s.PriceMonthly)

		result = append(result, Size{
			Slug:         s.Slug,
			Memory:       s.Memory,
			VCPUs:        s.Vcpus,
			Disk:         s.Disk,
			Transfer:     s.Transfer,
			PriceMonthly: s.PriceMonthly,
			Available:    s.Available,
			Regions:      s.Regions,
			Description:  description,
		})
	}

	return result, nil
}

// ListImages lists all available DigitalOcean images
func (d *DigitalOceanProvider) ListImages(ctx context.Context) ([]Image, error) {
	// Get only distribution images (not snapshots/backups)
	opt := &godo.ListOptions{PerPage: 200}
	images, _, err := d.client.Images.ListDistribution(ctx, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	result := make([]Image, 0, len(images))
	for _, img := range images {
		result = append(result, Image{
			Slug:         img.Slug,
			Name:         img.Name,
			Distribution: img.Distribution,
			Public:       img.Public,
		})
	}

	return result, nil
}
