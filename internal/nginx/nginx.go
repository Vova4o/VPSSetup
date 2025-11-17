package nginx

import (
	"fmt"
	"strings"
)

// SSHExecutor defines the interface for SSH command execution
type SSHExecutor interface {
	RunCommand(cmd string) (string, error)
}

// Service handles NGINX configuration
type Service struct {
	sshClient SSHExecutor
}

// NewService creates a new nginx service
func NewService(sshClient SSHExecutor) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// Config holds NGINX configuration
type Config struct {
	Domain      string
	Port        int
	SSL         bool
	CertPath    string
	KeyPath     string
	ProxyPass   string
	ConfigType  string // "static", "proxy", "php"
	RootPath    string
	IncludeWWW  bool
	MaxBodySize string
}

// Install installs NGINX package on the VPS
func (s *Service) Install() error {
	commands := []string{
		"apt-get update",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y nginx",
		"systemctl enable nginx",
		"systemctl start nginx",
	}

	for _, cmd := range commands {
		if _, err := s.sshClient.RunCommand(cmd); err != nil {
			return fmt.Errorf("failed to execute command '%s': %w", cmd, err)
		}
	}

	return nil
}

// GenerateConfig generates NGINX configuration based on type
func (s *Service) GenerateConfig(config Config) (string, error) {
	switch config.ConfigType {
	case "static":
		return s.generateStaticConfig(config), nil
	case "proxy":
		return s.generateProxyConfig(config), nil
	case "php":
		return s.generatePHPConfig(config), nil
	default:
		return "", fmt.Errorf("unsupported config type: %s", config.ConfigType)
	}
}

func (s *Service) generateStaticConfig(cfg Config) string {
	serverName := cfg.Domain
	if cfg.IncludeWWW {
		serverName = fmt.Sprintf("%s www.%s", cfg.Domain, cfg.Domain)
	}

	maxBodySize := cfg.MaxBodySize
	if maxBodySize == "" {
		maxBodySize = "10M"
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    root %s;
    index index.html index.htm;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ =404;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, cfg.RootPath, cfg.Domain, cfg.Domain, maxBodySize)

	if cfg.SSL {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root %s;
    index index.html index.htm;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    
    location / {
        try_files $uri $uri/ =404;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}`, serverName, cfg.CertPath, cfg.KeyPath, cfg.RootPath, cfg.Domain, cfg.Domain, maxBodySize)
	}

	return config
}

func (s *Service) generateProxyConfig(cfg Config) string {
	serverName := cfg.Domain
	if cfg.IncludeWWW {
		serverName = fmt.Sprintf("%s www.%s", cfg.Domain, cfg.Domain)
	}

	maxBodySize := cfg.MaxBodySize
	if maxBodySize == "" {
		maxBodySize = "10M"
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    location / {
        proxy_pass http://localhost:%d;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}`, serverName, cfg.Domain, cfg.Domain, maxBodySize, cfg.Port)

	if cfg.SSL {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    location / {
        proxy_pass http://localhost:%d;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}`, serverName, cfg.CertPath, cfg.KeyPath, cfg.Domain, cfg.Domain, maxBodySize, cfg.Port)
	}

	return config
}

func (s *Service) generatePHPConfig(cfg Config) string {
	serverName := cfg.Domain
	if cfg.IncludeWWW {
		serverName = fmt.Sprintf("%s www.%s", cfg.Domain, cfg.Domain)
	}

	maxBodySize := cfg.MaxBodySize
	if maxBodySize == "" {
		maxBodySize = "10M"
	}

	config := fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    
    server_name %s;
    
    root %s;
    index index.php index.html index.htm;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
    
    location ~ /\.ht {
        deny all;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
}`, serverName, cfg.RootPath, cfg.Domain, cfg.Domain, maxBodySize)

	if cfg.SSL {
		config += fmt.Sprintf(`

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name %s;
    
    ssl_certificate %s;
    ssl_certificate_key %s;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root %s;
    index index.php index.html index.htm;
    
    access_log /var/log/nginx/%s.access.log;
    error_log /var/log/nginx/%s.error.log;
    
    client_max_body_size %s;
    
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }
    
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
    
    location ~ /\.ht {
        deny all;
    }
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
}`, serverName, cfg.CertPath, cfg.KeyPath, cfg.RootPath, cfg.Domain, cfg.Domain, maxBodySize)
	}

	return config
}

// Deploy deploys NGINX configuration to VPS
func (s *Service) Deploy(domain, configContent, rootPath, configType string) error {
	// Create config file
	configPath := fmt.Sprintf("/etc/nginx/sites-available/%s", domain)

	// Escape single quotes in config content
	escapedConfig := strings.ReplaceAll(configContent, "'", "'\\''")

	uploadCmd := fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", escapedConfig, configPath)
	if _, err := s.sshClient.RunCommand(uploadCmd); err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}

	// Create symlink
	symlinkPath := fmt.Sprintf("/etc/nginx/sites-enabled/%s", domain)
	symlinkCmd := fmt.Sprintf("sudo ln -sf %s %s", configPath, symlinkPath)
	if _, err := s.sshClient.RunCommand(symlinkCmd); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Create document root if needed
	if rootPath != "" && (configType == "static" || configType == "php") {
		mkdirCmd := fmt.Sprintf("sudo mkdir -p %s && sudo chown -R www-data:www-data %s", rootPath, rootPath)
		if _, err := s.sshClient.RunCommand(mkdirCmd); err != nil {
			return fmt.Errorf("failed to create document root: %w", err)
		}

		// Create a default index.html for static sites
		if configType == "static" {
			indexContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Welcome</title>
</head>
<body>
    <h1>Welcome to %s</h1>
    <p>Your site is now running!</p>
</body>
</html>`, domain)
			escapedIndex := strings.ReplaceAll(indexContent, "'", "'\\''")
			indexCmd := fmt.Sprintf("echo '%s' | sudo tee %s/index.html > /dev/null", escapedIndex, rootPath)
			s.sshClient.RunCommand(indexCmd)
		}
	}

	// Test configuration
	if err := s.TestConfig(); err != nil {
		return err
	}

	// Reload NGINX
	return s.Reload()
}

// Reload reloads NGINX configuration
func (s *Service) Reload() error {
	reloadCmd := "sudo systemctl reload nginx"
	if _, err := s.sshClient.RunCommand(reloadCmd); err != nil {
		return fmt.Errorf("failed to reload nginx: %w", err)
	}
	return nil
}

// TestConfig tests NGINX configuration
func (s *Service) TestConfig() error {
	testCmd := "sudo nginx -t"
	output, err := s.sshClient.RunCommand(testCmd)
	if err != nil {
		return fmt.Errorf("nginx configuration test failed: %s: %w", output, err)
	}
	return nil
}
