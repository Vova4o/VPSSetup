package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SWebProvider handles sweb.ru API interactions
type SWebProvider struct {
	token  string
	client *http.Client
}

// NewSWebProvider creates a new sweb.ru provider
func NewSWebProvider(token string) *SWebProvider {
	return &SWebProvider{
		token:  token,
		client: &http.Client{},
	}
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      int             `json:"id,omitempty"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DNSRecord represents a DNS record
type DNSRecord struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	TTL    int    `json:"ttl,omitempty"`
	Domain string `json:"domain"`
}

// makeRequest makes a JSON-RPC request to sweb.ru API
func (s *SWebProvider) makeRequest(ctx context.Context, endpoint, method string, params interface{}) (*JSONRPCResponse, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := "https://api.sweb.ru/" + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jsonResp JSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if jsonResp.Error != nil {
		// Special handling for session expiration
		if jsonResp.Error.Code == -32603 && (jsonResp.Error.Message == "Время сеанса истекло." ||
			jsonResp.Error.Message == "Session expired") {
			return nil, fmt.Errorf("SWeb API session expired. Please generate a new API token from https://sweb.ru/cabinet/api/ and update your config")
		}
		return nil, fmt.Errorf("API error for method '%s': %s (code: %d)", method, jsonResp.Error.Message, jsonResp.Error.Code)
	}

	return &jsonResp, nil
}

// DNSRecordEntry represents a single DNS record from the sweb.ru API
// The API returns different fields depending on the record type
type DNSRecordEntry struct {
	Name      string `json:"name"`
	Value     string `json:"value"`
	Index     int    `json:"index"`
	Type      string `json:"type"`
	Category  string `json:"category"`
	CanChange string `json:"canChange,omitempty"`
	Sel       string `json:"sel,omitempty"`
	Priority  string `json:"priority,omitempty"`
	TTL       string `json:"ttl,omitempty"`
	Service   string `json:"service,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Weight    string `json:"weight,omitempty"`
	Port      string `json:"port,omitempty"`
	Target    string `json:"target,omitempty"`
	Domain    string `json:"domain,omitempty"` // For TXT records
	Main      int    `json:"main,omitempty"`
}

// AddDNSRecord adds a DNS record
// Note: sweb.ru API uses type-specific edit methods with action="add"
func (s *SWebProvider) AddDNSRecord(ctx context.Context, record DNSRecord) error {
	// Map record type to the appropriate edit method
	method := ""
	params := map[string]interface{}{
		"domain": record.Domain,
		"action": "add",
	}

	switch record.Type {
	case "A", "AAAA", "CNAME":
		// A/AAAA/CNAME records use the editMain method
		method = "editMain"
		params["name"] = record.Name
		params["type"] = record.Type
		params["value"] = record.Value
	case "TXT":
		method = "editTxt"
		params["subDomain"] = record.Name
		params["value"] = record.Value
	case "MX":
		method = "editMx"
		params["subDomain"] = record.Name
		params["value"] = record.Value
		if record.TTL > 0 {
			params["priority"] = record.TTL // MX uses priority instead of TTL
		}
	case "NS":
		method = "editNS"
		params["subDomain"] = record.Name
		params["value"] = record.Value
	case "SRV":
		method = "editSrv"
		params["subDomain"] = record.Name
		params["target"] = record.Value
		if record.TTL > 0 {
			params["ttl"] = record.TTL
		}
	default:
		return fmt.Errorf("unsupported record type: %s", record.Type)
	}

	_, err := s.makeRequest(ctx, "domains/dns", method, params)
	if err != nil {
		return fmt.Errorf("failed to add DNS record: %w", err)
	}

	return nil
}

// ListDNSRecords lists all DNS records for a domain
func (s *SWebProvider) ListDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	params := map[string]interface{}{
		"domain": domain,
	}

	resp, err := s.makeRequest(ctx, "domains/dns", "info", params)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}

	// The response is a flat array of record entries
	var entries []DNSRecordEntry
	if err := json.Unmarshal(resp.Result, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DNS records: %w", err)
	}

	// Convert API response to our DNSRecord format
	var records []DNSRecord
	for _, entry := range entries {
		// Determine the value based on record type
		value := entry.Value
		if entry.Target != "" {
			value = entry.Target
		}

		records = append(records, DNSRecord{
			ID:     fmt.Sprintf("%d", entry.Index),
			Name:   entry.Name,
			Type:   entry.Type,
			Value:  value,
			Domain: domain,
		})
	}

	return records, nil
}

// DeleteDNSRecord deletes a DNS record
func (s *SWebProvider) DeleteDNSRecord(ctx context.Context, domain, recordID, recordType, recordName, recordValue string) error {
	// Different methods for different record types
	var method string
	var endpoint string
	var params map[string]interface{}

	switch recordType {
	case "A", "AAAA", "CNAME":
		// A/AAAA/CNAME records are managed as subdomains via the domains API
		endpoint = "domains"
		method = "removeSubdomain"
		params = map[string]interface{}{
			"domain":  domain,
			"machine": recordName,
		}
	case "TXT":
		endpoint = "domains/dns"
		method = "editTxt"
		params = map[string]interface{}{
			"domain":    domain,
			"action":    "delete",
			"index":     recordID,
			"subDomain": recordName,
			"value":     recordValue,
		}
	case "MX":
		endpoint = "domains/dns"
		method = "editMx"
		params = map[string]interface{}{
			"domain":    domain,
			"action":    "delete",
			"index":     recordID,
			"subDomain": recordName,
			"value":     recordValue,
		}
	case "NS":
		endpoint = "domains/dns"
		method = "editNS"
		params = map[string]interface{}{
			"domain":    domain,
			"action":    "delete",
			"index":     recordID,
			"subDomain": recordName,
			"value":     recordValue,
		}
	case "SRV":
		endpoint = "domains/dns"
		method = "editSrv"
		params = map[string]interface{}{
			"domain":    domain,
			"action":    "delete",
			"index":     recordID,
			"subDomain": recordName,
			"target":    recordValue,
		}
	default:
		// Default to subdomain removal
		endpoint = "domains"
		method = "removeSubdomain"
		params = map[string]interface{}{
			"domain":  domain,
			"machine": recordName,
		}
	}

	_, err := s.makeRequest(ctx, endpoint, method, params)
	if err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	return nil
}

// UpdateDNSRecord updates a DNS record
func (s *SWebProvider) UpdateDNSRecord(ctx context.Context, record DNSRecord) error {
	method := ""
	params := map[string]interface{}{
		"domain": record.Domain,
		"action": "edit",
		"index":  record.ID,
	}

	switch record.Type {
	case "TXT":
		method = "editTxt"
		params["subDomain"] = record.Name
		params["value"] = record.Value
	case "MX":
		method = "editMx"
		params["subDomain"] = record.Name
		params["value"] = record.Value
		if record.TTL > 0 {
			params["priority"] = record.TTL
		}
	case "NS":
		method = "editNS"
		params["subDomain"] = record.Name
		params["value"] = record.Value
	case "SRV":
		method = "editSrv"
		params["subDomain"] = record.Name
		params["target"] = record.Value
		if record.TTL > 0 {
			params["ttl"] = record.TTL
		}
	default:
		return fmt.Errorf("unsupported record type for update: %s", record.Type)
	}

	_, err := s.makeRequest(ctx, "domains/dns", method, params)
	if err != nil {
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	return nil
}

// CreateSubdomain creates a subdomain using the domains API
// Note: This is for web hosting subdomains, but it also creates the DNS A record
// The subdomain will point to your hosting account's IP address
func (s *SWebProvider) CreateSubdomain(ctx context.Context, domain, subdomain, ipAddress string) error {
	params := map[string]interface{}{
		"domain":  domain,
		"machine": subdomain,
		"dir":     "/", // Default directory - required by API
	}

	// Note: The IP address parameter is not used in createSubdomain
	// The subdomain automatically points to the hosting account's IP
	// For custom IP addresses, you may need to modify the record after creation
	// or use the web control panel

	_, err := s.makeRequest(ctx, "domains", "createSubdomain", params)
	if err != nil {
		return fmt.Errorf("failed to create subdomain: %w", err)
	}

	return nil
}

// RemoveSubdomain removes a subdomain using the domains API
func (s *SWebProvider) RemoveSubdomain(ctx context.Context, domain, subdomain string) error {
	params := map[string]interface{}{
		"domain":  domain,
		"machine": subdomain,
	}

	_, err := s.makeRequest(ctx, "domains", "removeSubdomain", params)
	if err != nil {
		return fmt.Errorf("failed to remove subdomain: %w", err)
	}

	return nil
}

// ListRegions lists available regions (stub for SWeb)
func (s *SWebProvider) ListRegions(ctx context.Context) ([]Region, error) {
	// SWeb doesn't provide region selection
	return []Region{{Slug: "ru", Name: "Russia", Available: true}}, nil
}

// ListSizes lists available instance sizes (stub for SWeb)
func (s *SWebProvider) ListSizes(ctx context.Context) ([]Size, error) {
	// SWeb doesn't provide size selection via API
	return []Size{{Slug: "default", Description: "Default", Available: true}}, nil
}

// ListImages lists available OS images (stub for SWeb)
func (s *SWebProvider) ListImages(ctx context.Context) ([]Image, error) {
	// SWeb doesn't provide image selection
	return []Image{{Slug: "ubuntu-22-04", Name: "Ubuntu 22.04", Distribution: "Ubuntu", Public: true}}, nil
}

// ListDomains lists all domains in the SWeb account
func (s *SWebProvider) ListDomains(ctx context.Context) ([]string, error) {
	params := map[string]interface{}{}

	resp, err := s.makeRequest(ctx, "domains", "index", params)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	// SWeb returns array of domain objects directly
	// Try multiple possible field names
	var domainList []map[string]interface{}

	if err := json.Unmarshal(resp.Result, &domainList); err != nil {
		return nil, fmt.Errorf("failed to parse domain list: %w", err)
	}

	domains := make([]string, 0, len(domainList))
	for _, d := range domainList {
		// Try different field names that might contain the domain name
		if fqdn, ok := d["fqdn"].(string); ok && fqdn != "" {
			domains = append(domains, fqdn)
		} else if name, ok := d["name"].(string); ok && name != "" {
			domains = append(domains, name)
		} else if domain, ok := d["domain"].(string); ok && domain != "" {
			domains = append(domains, domain)
		} else if dname, ok := d["dname"].(string); ok && dname != "" {
			domains = append(domains, dname)
		}
	}

	return domains, nil
}
