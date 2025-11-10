package nginx

import (
	"bytes"
	"fmt"
	"text/template"
)

// Service handles NGINX configuration
type Service struct {
	sshClient interface{}
}

// NewService creates a new nginx service
func NewService(sshClient interface{}) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// Config holds NGINX configuration
type Config struct {
	Domain    string
	Port      int
	SSL       bool
	CertPath  string
	KeyPath   string
	ProxyPass string
}

// GenerateConfig generates NGINX configuration file
func (s *Service) GenerateConfig(config Config) (string, error) {
	// TODO: Implement config generation using templates
	const configTemplate = `
server {
    listen 80;
    server_name {{ .Domain }};

    {{ if .SSL }}
    listen 443 ssl http2;
    ssl_certificate {{ .CertPath }};
    ssl_certificate_key {{ .KeyPath }};
    {{ end }}

    location / {
        proxy_pass http://localhost:{{ .Port }};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
`
	tmpl, err := template.New("nginx").Parse(configTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Deploy deploys NGINX configuration to VPS
func (s *Service) Deploy(configContent string) error {
	// TODO: Implement deployment
	// 1. Upload config to /etc/nginx/sites-available/
	// 2. Create symlink to sites-enabled
	// 3. Test configuration
	// 4. Reload NGINX

	return fmt.Errorf("not implemented yet")
}

// Reload reloads NGINX configuration
func (s *Service) Reload() error {
	// TODO: Implement NGINX reload
	return fmt.Errorf("not implemented yet")
}

// TestConfig tests NGINX configuration
func (s *Service) TestConfig() error {
	// TODO: Implement config test (nginx -t)
	return fmt.Errorf("not implemented yet")
}
