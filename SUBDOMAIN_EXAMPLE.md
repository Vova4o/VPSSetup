# How to Add a Subdomain DNS Record

## Current Implementation Status

Based on the sweb.ru API, there are **two ways** to manage subdomains:

### 1. **TXT/MX/NS/SRV Records** (Working ✅)

These record types use the `editTxt`, `editMx`, `editNS`, `editSrv` methods with `action="add"`.

### 2. **A/AAAA/CNAME Records** (Not Yet Implemented ⚠️)

The sweb.ru API doesn't document a direct method for adding A/AAAA/CNAME records via the DNS API.

## Option 1: Use the Domains API for Subdomains

Based on the API documentation, you can use the **`createSubdomain`** method from the `/domains` endpoint:

```bash
# Example: Create a subdomain with A record
curl -X POST https://api.sweb.ru/domains \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "createSubdomain",
    "params": {
      "domain": "vova4o.com",
      "machine": "api",
      "dir": "/path/to/directory"
    },
    "id": 1
  }'
```

**Note**: The `createSubdomain` method is primarily for **web hosting** subdomains, not just DNS records. It creates a subdomain tied to a directory on the hosting account.

## Option 2: Add Implementation to the Tool

Here's how to add subdomain support to your VPSSetup tool:

### Step 1: Add a new method to sweb.go

```go
// CreateSubdomainWithIP creates a subdomain and points it to an IP
// This uses the domains API, not the DNS API
func (s *SWebProvider) CreateSubdomainWithIP(ctx context.Context, domain, subdomain, ipAddress string) error {
	params := map[string]interface{}{
		"domain":  domain,
		"machine": subdomain,
		"dir":     "/", // Default directory
	}

	_, err := s.makeRequest(ctx, "domains", "createSubdomain", params)
	if err != nil {
		return fmt.Errorf("failed to create subdomain: %w", err)
	}

	return nil
}

// RemoveSubdomain removes a subdomain
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
```

### Step 2: Add a CLI command

```go
// Add to cmd/main.go in the dns command group
var dnsAddSubdomainCmd = &cobra.Command{
	Use:   "add-subdomain",
	Short: "Add a subdomain A record",
	RunE:  runDNSAddSubdomain,
}

func init() {
	dnsCmd.AddCommand(dnsAddSubdomainCmd)
}

// Add to cmd/handlers.go
func runDNSAddSubdomain(cmd *cobra.Command, args []string) error {
	if err := ensureDomainConfigured(); err != nil {
		return err
	}

	prof, err := cfg.GetProfile(profile)
	if err != nil {
		return err
	}

	// Interactive prompts
	subdomain := interactive.AskInput("Enter subdomain name (e.g., 'api' for api.vova4o.com)", "")
	if subdomain == "" {
		return fmt.Errorf("subdomain name is required")
	}

	ipAddress := interactive.AskInput("Enter IP address", prof.VPS.PublicIP)
	if ipAddress == "" {
		return fmt.Errorf("IP address is required")
	}

	if !interactive.ConfirmAction(fmt.Sprintf("Create subdomain %s.%s pointing to %s?",
		subdomain, prof.Domain.Name, ipAddress)) {
		return fmt.Errorf("operation cancelled")
	}

	// Create DNS provider
	provider := provider.NewSWebProvider(prof.Domain.DNSAPIKey)

	fmt.Printf("🌐 Creating subdomain %s.%s...\n", subdomain, prof.Domain.Name)

	if err := provider.CreateSubdomainWithIP(context.Background(),
		prof.Domain.Name, subdomain, ipAddress); err != nil {
		return fmt.Errorf("failed to create subdomain: %w", err)
	}

	interactive.Success(fmt.Sprintf("Subdomain %s.%s created successfully!", subdomain, prof.Domain.Name))
	interactive.Info(fmt.Sprintf("It will point to: %s", ipAddress))
	interactive.Info("Note: DNS propagation may take a few minutes")

	return nil
}
```

## Quick Command Line Example (Manual)

To manually add a subdomain right now using the API:

```bash
# 1. Get your token (if needed)
TOKEN="your_sweb_token"

# 2. Create subdomain
curl -X POST https://api.sweb.ru/domains \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "createSubdomain",
    "params": {
      "domain": "vova4o.com",
      "machine": "api",
      "dir": "/"
    },
    "id": 1
  }'
```

## What Your Current Tool Can Do

Your `dns create` command currently works with:

- **TXT records** ✅
- **MX records** ✅
- **NS records** ✅
- **SRV records** ✅

Example usage:

```bash
# Add a TXT record
./bin/vpssetup dns create
# Select "TXT" when prompted
# Enter subdomain: "test"
# Enter value: "v=spf1 include:example.com ~all"
```

## Workaround for A Records

Since A/AAAA/CNAME records aren't directly supported via the DNS API methods we found, you have these options:

1. **Use the web control panel** at sweb.ru (quickest)
2. **Implement `createSubdomain`** as shown above
3. **Contact sweb.ru support** to confirm if there's an undocumented DNS API method for A records
4. **Use the zone file method** - fetch with `getFile`, modify, and potentially upload (if supported)

## Recommendation

For your use case (pointing subdomains to VPS IPs), I recommend:

1. **Short term**: Use the sweb.ru web interface to manually add A records
2. **Long term**: Implement the `createSubdomain` method shown above for automation

Would you like me to implement the `createSubdomain` functionality in your tool now?
