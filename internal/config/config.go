package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config represents the complete configuration
type Config struct {
	Profiles       map[string]*Profile `mapstructure:"profiles"`
	DefaultProfile string              `mapstructure:"default_profile"`
}

// Profile represents a deployment profile
type Profile struct {
	VPS     VPSConfig     `mapstructure:"vps"`
	Domain  DomainConfig  `mapstructure:"domain"`
	Project ProjectConfig `mapstructure:"project"`
	SSL     SSLConfig     `mapstructure:"ssl"`
	SSH     SSHConfig     `mapstructure:"ssh"`
}

// VPSConfig holds VPS provider configuration
type VPSConfig struct {
	Provider   string `mapstructure:"provider"`
	APIKey     string `mapstructure:"apikey"`
	Region     string `mapstructure:"region"`
	Size       string `mapstructure:"size"`
	InstanceID string `mapstructure:"instanceid"`
	PublicIP   string `mapstructure:"publicip"`
	Name       string `mapstructure:"name"`
	CreatedAt  string `mapstructure:"createdat"`
}

// DomainConfig holds domain configuration
type DomainConfig struct {
	Name        string `mapstructure:"name"`
	Subdomain   string `mapstructure:"subdomain"`
	DNSProvider string `mapstructure:"dnsprovider"`
	DNSAPIKey   string `mapstructure:"dnsapikey"`
}

// ProjectConfig holds project configuration
type ProjectConfig struct {
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
	Runtime string `mapstructure:"runtime"`
}

// SSLConfig holds SSL configuration
type SSLConfig struct {
	Email     string `mapstructure:"email"`
	Enable    bool   `mapstructure:"enable"`
	AutoRenew bool   `mapstructure:"autorenew"`
}

// SSHConfig holds SSH configuration
type SSHConfig struct {
	KeyPath string `mapstructure:"keypath"`
	User    string `mapstructure:"user"`
}

// Load loads configuration from file
func Load(configPath string) (*Config, error) {
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// GetProfile returns a profile by name, or the default profile if name is empty
func (c *Config) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}

	profile, exists := c.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return profile, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Profiles) == 0 {
		return fmt.Errorf("no profiles defined in config")
	}

	if c.DefaultProfile == "" {
		return fmt.Errorf("default_profile not specified")
	}

	if _, exists := c.Profiles[c.DefaultProfile]; !exists {
		return fmt.Errorf("default_profile '%s' does not exist", c.DefaultProfile)
	}

	return nil
}

// ValidateProfile validates a single profile
// Note: Only validates provider credentials and SSH settings
// Deployment-specific fields (region, size, domain, project) are validated at deploy time
func (p *Profile) Validate() error {
	// VPS validation - required for setup
	if p.VPS.Provider == "" {
		return fmt.Errorf("vps.provider is required")
	}
	if p.VPS.APIKey == "" {
		return fmt.Errorf("vps.api_key is required")
	}

	// DNS validation - required for setup
	if p.Domain.DNSProvider == "" {
		return fmt.Errorf("domain.dns_provider is required")
	}
	if p.Domain.DNSAPIKey == "" {
		return fmt.Errorf("domain.dns_api_key is required")
	}

	// SSH validation - required for setup
	if p.SSH.KeyPath == "" {
		return fmt.Errorf("ssh.key_path is required")
	}
	if p.SSH.User == "" {
		return fmt.Errorf("ssh.user is required")
	}

	// Deployment-specific validations (only if values are provided)
	if p.Domain.Name != "" && p.Project.Path != "" {
		// Region and size must be set for deployment
		if p.VPS.Region == "" {
			return fmt.Errorf("vps.region is required for deployment")
		}
		if p.VPS.Size == "" {
			return fmt.Errorf("vps.size is required for deployment")
		}
		if p.Project.Port == 0 {
			return fmt.Errorf("project.port is required for deployment")
		}
		if p.Project.Runtime == "" {
			return fmt.Errorf("project.runtime is required for deployment")
		}
		// Check if project path exists
		if _, err := os.Stat(p.Project.Path); os.IsNotExist(err) {
			return fmt.Errorf("project.path '%s' does not exist", p.Project.Path)
		}
	}

	return nil
}

// FullDomain returns the full domain (subdomain.domain)
func (p *Profile) FullDomain() string {
	if p.Domain.Subdomain == "" {
		return p.Domain.Name
	}
	return fmt.Sprintf("%s.%s", p.Domain.Subdomain, p.Domain.Name)
}

// Save saves the configuration back to the file
func (c *Config) Save(configPath string) error {
	if configPath == "" {
		configPath = "./config.yaml"
	}

	viper.Set("profiles", c.Profiles)
	viper.Set("default_profile", c.DefaultProfile)

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
