package nginx

import (
	"fmt"
	"strings"
	"text/template"
)

// ConfigType represents the type of NGINX configuration
type ConfigType string

const (
	ConfigTypeStatic       ConfigType = "static"
	ConfigTypeReverseProxy ConfigType = "proxy"
	ConfigTypeNodeJS       ConfigType = "nodejs"
	ConfigTypePHP          ConfigType = "php"
)

// Config represents NGINX configuration parameters
type Config struct {
	ServerName  string     // Domain name (e.g., example.com)
	ServerAlias []string   // Additional domains (e.g., www.example.com)
	ConfigType  ConfigType // Type of configuration
	RootPath    string     // Document root for static sites
	ProxyPort   int        // Port for reverse proxy
	EnableSSL   bool       // Whether to enable SSL
	SSLCertPath string     // Path to SSL certificate
	SSLKeyPath  string     // Path to SSL private key
	EnableGzip  bool       // Enable gzip compression
	MaxBodySize string     // Max request body size (e.g., "10M")
	AccessLog   string     // Access log path
	ErrorLog    string     // Error log path
	ExtraConfig string     // Additional custom configuration
}

// NewConfig creates a new NGINX configuration with defaults
func NewConfig(serverName string, configType ConfigType) *Config {
	return &Config{
		ServerName:  serverName,
		ServerAlias: []string{},
		ConfigType:  configType,
		RootPath:    fmt.Sprintf("/var/www/%s/html", serverName),
		ProxyPort:   3000,
		EnableSSL:   false,
		EnableGzip:  true,
		MaxBodySize: "10M",
		AccessLog:   fmt.Sprintf("/var/log/nginx/%s.access.log", serverName),
		ErrorLog:    fmt.Sprintf("/var/log/nginx/%s.error.log", serverName),
	}
}

// Generate generates the NGINX configuration file content
func (c *Config) Generate() (string, error) {
	var tmpl *template.Template
	var err error

	switch c.ConfigType {
	case ConfigTypeStatic:
		tmpl, err = template.New("static").Parse(staticTemplate)
	case ConfigTypeReverseProxy, ConfigTypeNodeJS:
		tmpl, err = template.New("proxy").Parse(reverseProxyTemplate)
	case ConfigTypePHP:
		tmpl, err = template.New("php").Parse(phpTemplate)
	default:
		return "", fmt.Errorf("unsupported config type: %s", c.ConfigType)
	}

	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, c); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GetConfigFilePath returns the path where the config should be saved on the server
func (c *Config) GetConfigFilePath() string {
	return fmt.Sprintf("/etc/nginx/sites-available/%s", c.ServerName)
}

// GetSymlinkPath returns the path for the enabled sites symlink
func (c *Config) GetSymlinkPath() string {
	return fmt.Sprintf("/etc/nginx/sites-enabled/%s", c.ServerName)
}

// GetServerAliasString returns the server_name directive with all aliases
func (c *Config) GetServerAliasString() string {
	if len(c.ServerAlias) == 0 {
		return c.ServerName
	}
	aliases := append([]string{c.ServerName}, c.ServerAlias...)
	return strings.Join(aliases, " ")
}

// Static site template
const staticTemplate = `server {
    listen 80;
    listen [::]:80;
    
    server_name {{ .GetServerAliasString }};
    
    root {{ .RootPath }};
    index index.html index.htm;
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
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
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- if .EnableSSL }}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name {{ .GetServerAliasString }};
    
    ssl_certificate {{ .SSLCertPath }};
    ssl_certificate_key {{ .SSLKeyPath }};
    
    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root {{ .RootPath }};
    index index.html index.htm;
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
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
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- end }}`

// Reverse proxy template (for Node.js, Python, Go applications)
const reverseProxyTemplate = `server {
    listen 80;
    listen [::]:80;
    
    server_name {{ .GetServerAliasString }};
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    location / {
        proxy_pass http://127.0.0.1:{{ .ProxyPort }};
        proxy_http_version 1.1;
        
        # Proxy headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- if .EnableSSL }}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name {{ .GetServerAliasString }};
    
    ssl_certificate {{ .SSLCertPath }};
    ssl_certificate_key {{ .SSLKeyPath }};
    
    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    location / {
        proxy_pass http://127.0.0.1:{{ .ProxyPort }};
        proxy_http_version 1.1;
        
        # Proxy headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- end }}`

// PHP-FPM template
const phpTemplate = `server {
    listen 80;
    listen [::]:80;
    
    server_name {{ .GetServerAliasString }};
    
    root {{ .RootPath }};
    index index.php index.html index.htm;
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
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
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- if .EnableSSL }}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    
    server_name {{ .GetServerAliasString }};
    
    ssl_certificate {{ .SSLCertPath }};
    ssl_certificate_key {{ .SSLKeyPath }};
    
    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    root {{ .RootPath }};
    index index.php index.html index.htm;
    
    access_log {{ .AccessLog }};
    error_log {{ .ErrorLog }};
    
    client_max_body_size {{ .MaxBodySize }};
    
    {{- if .EnableGzip }}
    
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/xml text/javascript application/x-javascript application/xml+rss application/json;
    {{- end }}
    
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
    
    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
    
    {{- if .ExtraConfig }}
    {{ .ExtraConfig }}
    {{- end }}
}
{{- end }}`
