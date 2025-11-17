package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Vova4o/VPSSetup/pkg/provider"
	"github.com/digitalocean/godo"
)

// handleGetDomains returns available domains from DNS provider
func (s *Server) handleGetDomains(w http.ResponseWriter, r *http.Request) {
	profile, err := s.getProfile(r)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Check if DNS provider is configured
	if profile.Domain.DNSProvider == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("DNS provider not configured"))
		return
	}

	if profile.Domain.DNSAPIKey == "" {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("DNS API key not configured"))
		return
	}

	ctx, cancel := withTimeout(r.Context(), timeoutShort)
	defer cancel()

	domains, err := s.fetchDomainsFromProvider(ctx, profile.Domain.DNSProvider, profile.Domain.DNSAPIKey)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, fmt.Errorf("failed to fetch domains: %w", err))
		return
	}

	// Debug logging
	fmt.Printf("📋 Fetched %d domains from %s: %v\n", len(domains), profile.Domain.DNSProvider, domains)

	s.respondSuccess(w, domains)
}

// fetchDomainsFromProvider fetches domains from the DNS provider
func (s *Server) fetchDomainsFromProvider(ctx context.Context, provider, apiKey string) ([]string, error) {
	switch provider {
	case "digitalocean":
		return s.fetchDigitalOceanDomains(ctx, apiKey)
	case "cloudflare":
		return s.fetchCloudflareDomains(ctx, apiKey)
	case "sweb", "sweb.ru":
		return s.fetchSWebDomains(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %s", provider)
	}
}

// fetchDigitalOceanDomains fetches domains from DigitalOcean
func (s *Server) fetchDigitalOceanDomains(ctx context.Context, apiKey string) ([]string, error) {
	// Create DigitalOcean client
	client := godo.NewFromToken(apiKey)

	domains, _, err := client.Domains.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list DigitalOcean domains: %w", err)
	}

	result := make([]string, len(domains))
	for i, d := range domains {
		result[i] = d.Name
	}

	return result, nil
}

// fetchCloudflareDomains fetches domains from Cloudflare
func (s *Server) fetchCloudflareDomains(ctx context.Context, apiKey string) ([]string, error) {
	// TODO: Implement Cloudflare API integration
	// For now, return a helpful message
	return []string{"cloudflare-integration-pending.com"}, nil
}

// fetchSWebDomains fetches domains from SWeb
func (s *Server) fetchSWebDomains(ctx context.Context, apiKey string) ([]string, error) {
	swebProvider := provider.NewSWebProvider(apiKey)
	return swebProvider.ListDomains(ctx)
}
