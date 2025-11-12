package dns

import (
	"context"
	"fmt"

	"github.com/Vova4o/VPSSetup/pkg/provider"
)

// Provider defines the interface for DNS providers
type Provider interface {
	AddDNSRecord(ctx context.Context, record provider.DNSRecord) error
	ListDNSRecords(ctx context.Context, domain string) ([]provider.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, domain, recordID, recordType, recordName, recordValue string) error
	UpdateDNSRecord(ctx context.Context, record provider.DNSRecord) error
}

// Service handles DNS operations
type Service struct {
	provider     Provider
	providerName string
	domain       string
}

// NewService creates a new DNS service
func NewService(providerName, apiKey, domain string) (*Service, error) {
	var dnsProvider Provider

	switch providerName {
	case "sweb.ru", "sweb":
		dnsProvider = provider.NewSWebProvider(apiKey)
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s (currently only sweb.ru is supported)", providerName)
	}

	return &Service{
		provider:     dnsProvider,
		providerName: providerName,
		domain:       domain,
	}, nil
}

// Record represents a DNS record
type Record struct {
	ID     string
	Type   string // A, AAAA, CNAME, TXT, MX, NS, SRV
	Name   string
	Value  string
	TTL    int
	Domain string
}

// CreateRecord creates a DNS record
func (s *Service) CreateRecord(ctx context.Context, record Record) error {
	// Convert to provider record format
	providerRecord := provider.DNSRecord{
		Domain: s.domain,
		Name:   record.Name,
		Type:   record.Type,
		Value:  record.Value,
		TTL:    record.TTL,
	}

	return s.provider.AddDNSRecord(ctx, providerRecord)
}

// UpdateRecord updates a DNS record (delete old + create new)
func (s *Service) UpdateRecord(ctx context.Context, oldRecord, newRecord Record) error {
	// For sweb.ru, we need to delete the old record and create a new one
	err := s.provider.DeleteDNSRecord(ctx, s.domain, oldRecord.ID, oldRecord.Type, oldRecord.Name, oldRecord.Value)
	if err != nil {
		return fmt.Errorf("failed to delete old record: %w", err)
	}

	// Create the new record
	providerRecord := provider.DNSRecord{
		Domain: s.domain,
		Name:   newRecord.Name,
		Type:   newRecord.Type,
		Value:  newRecord.Value,
		TTL:    newRecord.TTL,
	}

	err = s.provider.AddDNSRecord(ctx, providerRecord)
	if err != nil {
		return fmt.Errorf("failed to create new record: %w", err)
	}

	return nil
}

// DeleteRecord deletes a DNS record
func (s *Service) DeleteRecord(ctx context.Context, record Record) error {
	return s.provider.DeleteDNSRecord(ctx, s.domain, record.ID, record.Type, record.Name, record.Value)
}

// ListRecords lists all DNS records for the domain
func (s *Service) ListRecords(ctx context.Context) ([]Record, error) {
	providerRecords, err := s.provider.ListDNSRecords(ctx, s.domain)
	if err != nil {
		return nil, err
	}

	// Convert to internal record format
	records := make([]Record, len(providerRecords))
	for i, pr := range providerRecords {
		records[i] = Record{
			ID:     pr.ID,
			Type:   pr.Type,
			Name:   pr.Name,
			Value:  pr.Value,
			TTL:    pr.TTL,
			Domain: pr.Domain,
		}
	}

	return records, nil
}

// GetProviderName returns the name of the DNS provider
func (s *Service) GetProviderName() string {
	return s.providerName
}

// GetDomain returns the domain being managed
func (s *Service) GetDomain() string {
	return s.domain
}
