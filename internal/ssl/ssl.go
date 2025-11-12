package ssl

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SSHExecutor defines the interface for SSH command execution
type SSHExecutor interface {
	RunCommand(cmd string) (string, error)
}

// Service handles SSL certificate operations
type Service struct {
	sshClient SSHExecutor
}

// NewService creates a new SSL service
func NewService(sshClient SSHExecutor) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// Config holds SSL configuration
type Config struct {
	Domain    string
	Email     string
	AutoRenew bool
	Webroot   string // Optional: path to webroot (default: /var/www/html)
}

// Install installs Let's Encrypt certificate
func (s *Service) Install(config Config) error {
	fmt.Printf("Installing SSL certificate for %s...\n", config.Domain)

	// Set default webroot if not provided
	if config.Webroot == "" {
		config.Webroot = "/var/www/html"
	}

	// 1. Install certbot if not already installed
	fmt.Println("📦 Installing certbot...")
	installCmd := `
	if ! command -v certbot &> /dev/null; then
		apt-get update && apt-get install -y certbot python3-certbot-nginx
	else
		echo "certbot already installed"
	fi
	`
	output, err := s.sshClient.RunCommand(installCmd)
	if err != nil {
		return fmt.Errorf("failed to install certbot: %w (output: %s)", err, output)
	}

	// 2. Stop NGINX temporarily to allow certbot standalone mode
	fmt.Println("🛑 Stopping NGINX temporarily...")
	_, err = s.sshClient.RunCommand("systemctl stop nginx")
	if err != nil {
		return fmt.Errorf("failed to stop nginx: %w", err)
	}

	// 3. Run certbot to obtain certificate
	fmt.Println("🔐 Obtaining SSL certificate...")
	certbotCmd := fmt.Sprintf(
		"certbot certonly --standalone --non-interactive --agree-tos --email %s -d %s",
		config.Email,
		config.Domain,
	)
	output, err = s.sshClient.RunCommand(certbotCmd)
	if err != nil {
		// Try to restart NGINX even if certbot fails
		s.sshClient.RunCommand("systemctl start nginx")
		return fmt.Errorf("failed to obtain certificate: %w (output: %s)", err, output)
	}

	// 4. Configure NGINX to use the certificate
	fmt.Println("⚙️  Configuring NGINX...")
	nginxConfigCmd := fmt.Sprintf(`
	# Update NGINX config to use SSL
	NGINX_CONF="/etc/nginx/sites-available/default"
	if [ -f "$NGINX_CONF" ]; then
		# Backup original config
		cp $NGINX_CONF ${NGINX_CONF}.backup
		
		# Add SSL configuration
		cat > /etc/nginx/sites-available/%s << 'EOF'
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    root /var/www/html;
    index index.html index.htm index.nginx-debian.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
EOF
		ln -sf /etc/nginx/sites-available/%s /etc/nginx/sites-enabled/%s
	fi
	`, config.Domain, config.Domain, config.Domain, config.Domain, config.Domain, config.Domain, config.Domain)

	_, err = s.sshClient.RunCommand(nginxConfigCmd)
	if err != nil {
		return fmt.Errorf("failed to configure nginx: %w", err)
	}

	// 5. Test NGINX configuration
	fmt.Println("🧪 Testing NGINX configuration...")
	_, err = s.sshClient.RunCommand("nginx -t")
	if err != nil {
		return fmt.Errorf("nginx configuration test failed: %w", err)
	}

	// 6. Start NGINX
	fmt.Println("🚀 Starting NGINX...")
	_, err = s.sshClient.RunCommand("systemctl start nginx")
	if err != nil {
		return fmt.Errorf("failed to start nginx: %w", err)
	}

	// 7. Configure auto-renewal if enabled
	if config.AutoRenew {
		fmt.Println("⏰ Configuring auto-renewal...")
		autoRenewCmd := `
		# Certbot usually sets up auto-renewal via systemd timer
		systemctl enable certbot.timer
		systemctl start certbot.timer
		`
		_, err = s.sshClient.RunCommand(autoRenewCmd)
		if err != nil {
			return fmt.Errorf("failed to configure auto-renewal: %w", err)
		}
	}

	fmt.Println("✅ SSL certificate installed successfully!")
	return nil
}

// Renew renews SSL certificate
func (s *Service) Renew(domain string) error {
	fmt.Printf("Renewing SSL certificate for %s...\n", domain)

	// Run certbot renew
	renewCmd := "certbot renew --quiet"
	output, err := s.sshClient.RunCommand(renewCmd)
	if err != nil {
		return fmt.Errorf("failed to renew certificate: %w (output: %s)", err, output)
	}

	// Reload NGINX to use new certificate
	_, err = s.sshClient.RunCommand("systemctl reload nginx")
	if err != nil {
		return fmt.Errorf("failed to reload nginx: %w", err)
	}

	fmt.Println("✅ SSL certificate renewed successfully!")
	return nil
}

// Status checks SSL certificate status
func (s *Service) Status(domain string) (*CertificateInfo, error) {
	// Check if certificate exists and get its information
	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)

	// Check if certificate file exists
	checkCmd := fmt.Sprintf("[ -f %s ] && echo 'exists' || echo 'not found'", certPath)
	output, err := s.sshClient.RunCommand(checkCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to check certificate: %w", err)
	}

	if strings.TrimSpace(output) != "exists" {
		return nil, fmt.Errorf("certificate not found for domain %s", domain)
	}

	// Get certificate information using openssl
	certInfoCmd := fmt.Sprintf(`
	openssl x509 -in %s -noout -subject -issuer -dates
	`, certPath)

	output, err = s.sshClient.RunCommand(certInfoCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate info: %w", err)
	}

	// Parse certificate information
	info := &CertificateInfo{
		Domain: domain,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "issuer=") {
			info.Issuer = strings.TrimPrefix(line, "issuer=")
		} else if strings.HasPrefix(line, "notBefore=") {
			info.ValidFrom = strings.TrimPrefix(line, "notBefore=")
		} else if strings.HasPrefix(line, "notAfter=") {
			info.ValidUntil = strings.TrimPrefix(line, "notAfter=")
			// Calculate days left
			if validUntil, err := time.Parse("Jan 2 15:04:05 2006 MST", info.ValidUntil); err == nil {
				daysLeft := int(time.Until(validUntil).Hours() / 24)
				info.DaysLeft = daysLeft
			}
		}
	}

	// Get expiration in days using certbot
	expiryCmd := fmt.Sprintf("certbot certificates -d %s 2>/dev/null | grep -oP 'VALID: \\K[0-9]+' || echo '0'", domain)
	output, err = s.sshClient.RunCommand(expiryCmd)
	if err == nil && strings.TrimSpace(output) != "0" {
		if days, err := strconv.Atoi(strings.TrimSpace(output)); err == nil {
			info.DaysLeft = days
		}
	}

	return info, nil
}

// CheckAutoRenewal checks if auto-renewal is configured
func (s *Service) CheckAutoRenewal() (bool, error) {
	// Check if certbot timer is active
	output, err := s.sshClient.RunCommand("systemctl is-active certbot.timer")
	if err != nil {
		return false, nil
	}

	return strings.TrimSpace(output) == "active", nil
}

// TestRenewal tests the renewal process without actually renewing
func (s *Service) TestRenewal() error {
	output, err := s.sshClient.RunCommand("certbot renew --dry-run")
	if err != nil {
		return fmt.Errorf("renewal test failed: %w (output: %s)", err, output)
	}
	return nil
}

// CertificateInfo holds certificate information
type CertificateInfo struct {
	Domain     string
	Issuer     string
	ValidFrom  string
	ValidUntil string
	DaysLeft   int
}
