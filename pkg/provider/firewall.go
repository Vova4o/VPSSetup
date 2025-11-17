package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/digitalocean/godo"
)

// FirewallRule represents a firewall rule
type FirewallRule struct {
	Protocol  string
	PortRange string
	Sources   []string // IP addresses or "0.0.0.0/0" for all
}

// CreateFirewall creates a firewall with the specified rules and applies it to the instance
func (d *DigitalOceanProvider) CreateFirewall(ctx context.Context, name string, instanceID string, rules []FirewallRule) (string, error) {
	dropletID, err := strconv.Atoi(instanceID)
	if err != nil {
		return "", fmt.Errorf("invalid instance ID: %w", err)
	}

	inboundRules := make([]godo.InboundRule, 0, len(rules))
	for _, rule := range rules {
		sources := &godo.Sources{}
		for _, source := range rule.Sources {
			sources.Addresses = append(sources.Addresses, source)
		}

		inboundRules = append(inboundRules, godo.InboundRule{
			Protocol:  rule.Protocol,
			PortRange: rule.PortRange,
			Sources:   sources,
		})
	}

	// Allow all outbound traffic by default
	outboundRules := []godo.OutboundRule{
		{
			Protocol:  "tcp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol:  "udp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol: "icmp",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
	}

	createRequest := &godo.FirewallRequest{
		Name:          name,
		InboundRules:  inboundRules,
		OutboundRules: outboundRules,
		DropletIDs:    []int{dropletID},
	}

	firewall, _, err := d.client.Firewalls.Create(ctx, createRequest)
	if err != nil {
		return "", fmt.Errorf("failed to create firewall: %w", err)
	}

	return firewall.ID, nil
}

// GetDefaultFirewallRules returns standard firewall rules for a web server
func GetDefaultFirewallRules() []FirewallRule {
	return []FirewallRule{
		{
			Protocol:  "tcp",
			PortRange: "22",
			Sources:   []string{"0.0.0.0/0", "::/0"}, // SSH from anywhere
		},
		{
			Protocol:  "tcp",
			PortRange: "80",
			Sources:   []string{"0.0.0.0/0", "::/0"}, // HTTP from anywhere
		},
		{
			Protocol:  "tcp",
			PortRange: "443",
			Sources:   []string{"0.0.0.0/0", "::/0"}, // HTTPS from anywhere
		},
	}
}

// DeleteFirewall deletes a firewall
func (d *DigitalOceanProvider) DeleteFirewall(ctx context.Context, firewallID string) error {
	_, err := d.client.Firewalls.Delete(ctx, firewallID)
	if err != nil {
		return fmt.Errorf("failed to delete firewall: %w", err)
	}
	return nil
}

// AddDropletToFirewall adds a droplet to an existing firewall
func (d *DigitalOceanProvider) AddDropletToFirewall(ctx context.Context, firewallID, instanceID string) error {
	dropletID, err := strconv.Atoi(instanceID)
	if err != nil {
		return fmt.Errorf("invalid instance ID: %w", err)
	}

	_, err = d.client.Firewalls.AddDroplets(ctx, firewallID, dropletID)
	if err != nil {
		return fmt.Errorf("failed to add droplet to firewall: %w", err)
	}
	return nil
}
