package interactive

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Vova4o/VPSSetup/internal/config"
	"github.com/briandowns/spinner"
	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// ConfirmAction asks for user confirmation
func ConfirmAction(message string) (bool, error) {
	confirm := false
	prompt := &survey.Confirm{
		Message: message,
		Default: false,
	}
	err := survey.AskOne(prompt, &confirm)
	return confirm, err
}

// SelectProfile prompts user to select a profile
func SelectProfile(cfg *config.Config) (string, error) {
	profiles := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		profiles = append(profiles, name)
	}

	var selected string
	prompt := &survey.Select{
		Message: "Select a profile:",
		Options: profiles,
		Default: cfg.DefaultProfile,
	}
	err := survey.AskOne(prompt, &selected)
	return selected, err
}

// SetupWizard runs an interactive setup wizard
func SetupWizard() (*config.Profile, error) {
	profile := &config.Profile{}

	fmt.Println("🚀 VPS Setup Wizard")
	fmt.Println("Let's configure your VPS deployment!")

	// Step 1: Collect API Keys First
	fmt.Println("📋 Step 1: API Credentials")
	fmt.Println("   We need your API keys to fetch available options from providers")

	// VPS Provider
	providerPrompt := &survey.Select{
		Message: "Select VPS provider:",
		Options: []string{"digitalocean"},
	}
	survey.AskOne(providerPrompt, &profile.VPS.Provider)

	// VPS API Key
	for profile.VPS.APIKey == "" {
		apiKeyPrompt := &survey.Password{
			Message: fmt.Sprintf("Enter %s API key:", profile.VPS.Provider),
			Help:    "Get your API key from your provider's dashboard",
		}
		survey.AskOne(apiKeyPrompt, &profile.VPS.APIKey)

		if profile.VPS.APIKey == "" {
			Warning("API key cannot be empty!")
		}
	}

	// DNS Provider
	dnsProviderPrompt := &survey.Select{
		Message: "Select DNS provider:",
		Options: []string{"sweb.ru"},
	}
	survey.AskOne(dnsProviderPrompt, &profile.Domain.DNSProvider)

	for profile.Domain.DNSAPIKey == "" {
		dnsAPIKeyPrompt := &survey.Password{
			Message: fmt.Sprintf("Enter %s API key/token:", profile.Domain.DNSProvider),
			Help:    "Required for automatic subdomain registration",
		}
		survey.AskOne(dnsAPIKeyPrompt, &profile.Domain.DNSAPIKey)

		if profile.Domain.DNSAPIKey == "" {
			Warning("DNS API key cannot be empty!")
		}
	}

	// Step 2: SSH Configuration
	fmt.Println("\n🔑 Step 2: SSH Configuration")

	// Default SSH key path
	defaultKeyPath := "~/.ssh/vpssetup_rsa"
	expandedPath, _ := expandPath(defaultKeyPath)

	// Check if SSH key exists
	if !fileExists(expandedPath) {
		Info("SSH key not found. Let's create one!")

		createKey, err := ConfirmAction("Create new SSH key pair?")
		if err != nil {
			return nil, err
		}

		if createKey {
			s := ShowSpinner("Generating SSH key pair...")
			if err := generateSSHKey(expandedPath); err != nil {
				s.Stop()
				Warning(fmt.Sprintf("Failed to generate SSH key: %v", err))
				Info("You can create one manually with: ssh-keygen -t rsa -b 4096")
			} else {
				s.Stop()
				Success(fmt.Sprintf("SSH key created: %s", defaultKeyPath))
				Success(fmt.Sprintf("Public key: %s.pub", defaultKeyPath))
				profile.SSH.KeyPath = defaultKeyPath
			}
		} else {
			// Ask for existing key path
			sshKeyPrompt := &survey.Input{
				Message: "Enter SSH key path:",
				Default: "~/.ssh/id_rsa",
				Help:    "Path to your SSH private key for server access",
			}
			survey.AskOne(sshKeyPrompt, &profile.SSH.KeyPath)
		}
	} else {
		Success(fmt.Sprintf("SSH key found: %s", defaultKeyPath))
		profile.SSH.KeyPath = defaultKeyPath
	}

	// SSH user is always root for initial setup
	profile.SSH.User = "root"
	Info("SSH user set to: root")

	// Set default values for deployment-time fields
	// These will be asked during actual deployment
	profile.VPS.Region = "" // Will ask during deploy
	profile.VPS.Size = ""   // Will ask during deploy
	profile.Domain.Name = ""
	profile.Domain.Subdomain = ""
	profile.Project.Path = ""
	profile.Project.Port = 0
	profile.Project.Runtime = ""
	profile.SSL.Enable = true
	profile.SSL.AutoRenew = true
	profile.SSL.Email = ""

	fmt.Println("\n✨ Configuration complete!")
	Info("Region, size, domain, project, and SSL details will be asked when you deploy")

	return profile, nil
}

// ShowSpinner shows a spinner with a message
func ShowSpinner(message string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + message
	s.Start()
	return s
}

// Success shows a success message
func Success(message string) {
	fmt.Printf("✅ %s\n", message)
}

// Error shows an error message
func Error(message string) {
	fmt.Printf("❌ %s\n", message)
}

// Info shows an info message
func Info(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}

// Warning shows a warning message
func Warning(message string) {
	fmt.Printf("⚠️  %s\n", message)
}

// getRegionsForProvider returns regions for a provider
// TODO: This should fetch real data from provider API using the API key
func getRegionsForProvider(provider string) []string {
	// Placeholder: In real implementation, we'd call the provider API
	// Example: client := digitalocean.New(apiKey); regions := client.ListRegions()

	regions := map[string][]string{
		"digitalocean": {"nyc1 (New York)", "nyc3 (New York)", "sfo3 (San Francisco)", "sgp1 (Singapore)", "lon1 (London)", "fra1 (Frankfurt)", "tor1 (Toronto)"},
		"linode":       {"us-east (Newark)", "us-west (Fremont)", "eu-west (London)", "ap-south (Singapore)", "ap-northeast (Tokyo)"},
		"vultr":        {"ewr (New Jersey)", "ord (Chicago)", "dfw (Dallas)", "sea (Seattle)", "lax (Los Angeles)", "atl (Atlanta)", "ams (Amsterdam)", "lhr (London)", "fra (Frankfurt)"},
		"hetzner":      {"nbg1 (Nuremberg)", "fsn1 (Falkenstein)", "hel1 (Helsinki)", "ash (Ashburn)"},
	}

	if r, ok := regions[provider]; ok {
		return r
	}
	return []string{"default"}
}

// getSizesForProvider returns instance sizes for a provider
// TODO: This should fetch real data from provider API using the API key
func getSizesForProvider(provider string) []string {
	// Placeholder: In real implementation, we'd call the provider API
	// Example: client := digitalocean.New(apiKey); sizes := client.ListSizes()

	sizes := map[string][]string{
		"digitalocean": {"s-1vcpu-1gb ($6/mo)", "s-2vcpu-2gb ($12/mo)", "s-2vcpu-4gb ($24/mo)", "s-4vcpu-8gb ($48/mo)"},
		"linode":       {"g6-nanode-1 ($5/mo)", "g6-standard-1 ($10/mo)", "g6-standard-2 ($20/mo)", "g6-standard-4 ($40/mo)"},
		"vultr":        {"vc2-1c-1gb ($6/mo)", "vc2-1c-2gb ($12/mo)", "vc2-2c-4gb ($24/mo)", "vc2-4c-8gb ($48/mo)"},
		"hetzner":      {"cx11 (€4.51/mo)", "cx21 (€5.83/mo)", "cx31 (€11.05/mo)", "cx41 (€16.14/mo)"},
	}

	if s, ok := sizes[provider]; ok {
		return s
	}
	return []string{"small", "medium", "large"}
}

// AskMultiline prompts for multiline input
func AskMultiline(message string) (string, error) {
	var result string
	prompt := &survey.Multiline{
		Message: message,
	}
	err := survey.AskOne(prompt, &result)
	return result, err
}

// AskInput prompts for simple input
func AskInput(message, defaultValue string) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}
	err := survey.AskOne(prompt, &result)
	return result, err
}

// AskSelect prompts for selection from a list
func AskSelect(message string, options []string) (string, error) {
	var result string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}
	err := survey.AskOne(prompt, &result)
	return result, err
}

// AskVPSConfiguration asks for VPS configuration (region and size only)
func AskVPSConfiguration(profile *config.Profile) error {
	fmt.Println(" VPS Configuration")

	s := ShowSpinner("Fetching available regions from DigitalOcean...")
	regions, err := fetchDigitalOceanRegions(profile.VPS.APIKey)
	s.Stop()
	if err != nil {
		Warning(fmt.Sprintf("Failed to fetch regions: %v", err))
		Info("Using default region list")
		regions = getRegionsForProvider(profile.VPS.Provider)
	} else {
		Success("Regions loaded successfully")
	}

	regionPrompt := &survey.Select{
		Message: "Select region:",
		Options: regions,
		Help:    "Choose a region closest to your users for better performance",
	}
	if err := survey.AskOne(regionPrompt, &profile.VPS.Region); err != nil {
		return err
	}

	s = ShowSpinner("Fetching available instance sizes...")
	sizes, err := fetchDigitalOceanSizes(profile.VPS.APIKey)
	s.Stop()
	if err != nil {
		Warning(fmt.Sprintf("Failed to fetch sizes: %v", err))
		Info("Using default size list")
		sizes = getSizesForProvider(profile.VPS.Provider)
	} else {
		Success("Instance sizes loaded successfully")
	}

	sizePrompt := &survey.Select{
		Message: "Select instance size:",
		Options: sizes,
		Help:    "Start small, you can always upgrade later",
	}
	if err := survey.AskOne(sizePrompt, &profile.VPS.Size); err != nil {
		return err
	}

	return nil
}

// AskDeploymentDetails asks for deployment-specific details
func AskDeploymentDetails(profile *config.Profile) error {
	fmt.Println("\n🚀 Deployment Configuration")
	fmt.Println("   These settings are specific to this deployment")

	// VPS Configuration - reuse the function
	if err := AskVPSConfiguration(profile); err != nil {
		return err
	}

	fmt.Println()

	// Domain
	domainPrompt := &survey.Input{
		Message: "Enter your domain name:",
		Help:    "Example: example.com",
	}
	if err := survey.AskOne(domainPrompt, &profile.Domain.Name); err != nil {
		return err
	}

	subdomainPrompt := &survey.Input{
		Message: "Enter subdomain (leave empty for root domain):",
		Help:    "Example: app (will create app.example.com)",
	}
	if err := survey.AskOne(subdomainPrompt, &profile.Domain.Subdomain); err != nil {
		return err
	}

	// Project
	projectPathPrompt := &survey.Input{
		Message: "Enter project path:",
		Default: "./",
		Help:    "Path to your project directory",
	}
	if err := survey.AskOne(projectPathPrompt, &profile.Project.Path); err != nil {
		return err
	}

	projectPortPrompt := &survey.Input{
		Message: "Enter application port:",
		Default: "8080",
	}
	var portStr string
	if err := survey.AskOne(projectPortPrompt, &portStr); err != nil {
		return err
	}
	fmt.Sscanf(portStr, "%d", &profile.Project.Port)

	runtimePrompt := &survey.Select{
		Message: "Select runtime:",
		Options: []string{"go", "nodejs", "python", "php", "ruby", "static"},
	}
	if err := survey.AskOne(runtimePrompt, &profile.Project.Runtime); err != nil {
		return err
	}

	// SSL
	sslPrompt := &survey.Confirm{
		Message: "Enable SSL/HTTPS?",
		Default: true,
		Help:    "Recommended for production. Uses Let's Encrypt for free certificates",
	}
	if err := survey.AskOne(sslPrompt, &profile.SSL.Enable); err != nil {
		return err
	}

	if profile.SSL.Enable {
		emailPrompt := &survey.Input{
			Message: "Enter email for Let's Encrypt:",
			Help:    "Required for SSL certificate registration",
		}
		if err := survey.AskOne(emailPrompt, &profile.SSL.Email); err != nil {
			return err
		}

		autoRenewPrompt := &survey.Confirm{
			Message: "Enable automatic SSL renewal?",
			Default: true,
		}
		if err := survey.AskOne(autoRenewPrompt, &profile.SSL.AutoRenew); err != nil {
			return err
		}
	}

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, path[1:]), nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// generateSSHKey generates a new SSH key pair
func generateSSHKey(keyPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate private key using ssh-keygen command
	cmd := exec.Command("ssh-keygen",
		"-t", "rsa",
		"-b", "4096",
		"-f", keyPath,
		"-N", "", // No passphrase
		"-C", "vpssetup-key",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen failed: %w, output: %s", err, string(output))
	}

	// Set proper permissions
	if err := os.Chmod(keyPath, 0o600); err != nil {
		return fmt.Errorf("failed to set key permissions: %w", err)
	}
	if err := os.Chmod(keyPath+".pub", 0o644); err != nil {
		return fmt.Errorf("failed to set public key permissions: %w", err)
	}

	return nil
}

// fetchDigitalOceanRegions fetches available regions from DigitalOcean API
func fetchDigitalOceanRegions(apiKey string) ([]string, error) {
	ctx := context.Background()

	// Create OAuth2 token source
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
	oauthClient := oauth2.NewClient(ctx, tokenSource)

	// Create DigitalOcean client
	client := godo.NewClient(oauthClient)

	// Fetch regions
	regions, _, err := client.Regions.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, err
	}

	// Format region options
	var options []string
	for _, region := range regions {
		if region.Available {
			options = append(options, fmt.Sprintf("%s (%s)", region.Slug, region.Name))
		}
	}

	return options, nil
}

// fetchDigitalOceanSizes fetches available instance sizes from DigitalOcean API
func fetchDigitalOceanSizes(apiKey string) ([]string, error) {
	ctx := context.Background()

	// Create OAuth2 token source
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})
	oauthClient := oauth2.NewClient(ctx, tokenSource)

	// Create DigitalOcean client
	client := godo.NewClient(oauthClient)

	// Fetch sizes
	sizes, _, err := client.Sizes.List(ctx, &godo.ListOptions{PerPage: 200})
	if err != nil {
		return nil, err
	}

	// Format size options (filter for available and reasonable sizes)
	var options []string
	for _, size := range sizes {
		if size.Available && size.Memory >= 512 {
			options = append(options, fmt.Sprintf("%s (%dGB RAM, %d vCPUs, $%.2f/mo)",
				size.Slug, size.Memory/1024, size.Vcpus, size.PriceMonthly))
		}
	}

	return options, nil
}
