# NGINX Configuration Guide

This guide explains how to use the NGINX configuration feature of VPSSetup.

## Overview

VPSSetup automates NGINX configuration for three common use cases:

1. **Static Websites** - HTML/CSS/JavaScript sites
2. **Reverse Proxy** - Node.js, Go, Python applications
3. **PHP Applications** - PHP-FPM based apps

## Quick Start

```bash
./bin/vpssetup nginx setup
```

This interactive wizard will:

1. Install NGINX if not already present
2. Ask for your domain name
3. Let you choose configuration type
4. Generate optimized NGINX config
5. Deploy and test the configuration
6. Create necessary directories
7. Reload NGINX

## Configuration Types

### 1. Static Website

Perfect for:

- HTML/CSS/JavaScript sites
- Single-page applications (React, Vue, Angular)
- Static site generators (Hugo, Jekyll, Gatsby)

Features:

- Gzip compression enabled
- Security headers (X-Frame-Options, X-XSS-Protection, etc.)
- Static asset caching (1 year for images, CSS, JS)
- Try-files directive for clean URLs

Example structure:

```
/var/www/example.com/html/
├── index.html
├── css/
├── js/
└── images/
```

### 2. Reverse Proxy

Perfect for:

- Node.js/Express applications
- Go web servers
- Python/Flask or Django apps
- Any backend running on localhost

Features:

- WebSocket support (upgrade headers)
- Proper proxy headers (X-Real-IP, X-Forwarded-For)
- Gzip compression for proxied content
- 60s timeouts (configurable)
- HTTP/1.1 with keep-alive

Default setup proxies to `http://127.0.0.1:3000`

### 3. PHP Application

Perfect for:

- WordPress
- Laravel applications
- Custom PHP sites
- PHP frameworks

Features:

- PHP-FPM integration via Unix socket
- Security (deny .ht\* files)
- Try-files for clean URLs
- Static asset caching
- FastCGI parameter support

Default setup uses `/var/run/php/php-fpm.sock`

## SSL/TLS Support

The wizard asks if you want SSL configuration. If you say "yes", it will:

- Generate both HTTP (port 80) and HTTPS (port 443) server blocks
- Configure SSL protocols (TLSv1.2, TLSv1.3)
- Add HSTS header for HTTPS
- Use Let's Encrypt certificate paths

**Note:** You must obtain SSL certificates separately (e.g., with certbot).

Example paths used:

```
/etc/letsencrypt/live/example.com/fullchain.pem
/etc/letsencrypt/live/example.com/privkey.pem
```

## Commands

### Setup NGINX Configuration

```bash
./bin/vpssetup nginx setup
```

Interactive wizard to create and deploy NGINX configuration.

### Test Configuration

```bash
./bin/vpssetup nginx test
```

Runs `nginx -t` on the VPS to check for syntax errors.

### Reload NGINX

```bash
./bin/vpssetup nginx reload
```

Reloads NGINX to apply configuration changes without downtime.

## Generated Configuration

### File Locations

- **Config file**: `/etc/nginx/sites-available/<domain>`
- **Enabled symlink**: `/etc/nginx/sites-enabled/<domain>`
- **Access log**: `/var/log/nginx/<domain>.access.log`
- **Error log**: `/var/log/nginx/<domain>.error.log`

### Security Features

All configurations include:

- **X-Frame-Options**: Prevents clickjacking
- **X-Content-Type-Options**: Prevents MIME sniffing
- **X-XSS-Protection**: Enables XSS filter
- **HSTS** (HTTPS only): Forces HTTPS for 1 year

### Performance Features

- **Gzip compression**: Reduces bandwidth by 50-70%
- **Static asset caching**: 1-year cache for images, fonts, CSS, JS
- **HTTP/2**: Enabled for SSL configurations
- **Keep-alive**: Connection reuse for better performance

## Examples

### Complete Static Site Setup

```bash
# 1. Create VPS and harden security
./bin/vpssetup setup
./bin/vpssetup harden

# 2. Configure NGINX
./bin/vpssetup nginx setup
# Choose: static
# Domain: mysite.com
# Include www: yes
# Path: /var/www/mysite.com/html

# 3. Upload your files (via SCP or other method)
scp -r ./dist/* root@<vps-ip>:/var/www/mysite.com/html/

# 4. Create DNS record
./bin/vpssetup dns create
# Type: A
# Name: @ (or subdomain)
# Value: <vps-ip>

# 5. Test
curl http://mysite.com
```

### Node.js API with Reverse Proxy

```bash
# 1. Setup VPS
./bin/vpssetup setup
./bin/vpssetup harden

# 2. Configure NGINX
./bin/vpssetup nginx setup
# Choose: proxy
# Domain: api.mysite.com
# Include www: no
# Port: 3000

# 3. Deploy your Node.js app
# (upload code, install dependencies, start app on port 3000)

# 4. Create DNS
./bin/vpssetup dns create
# Type: A
# Name: api
# Value: <vps-ip>

# 5. Test
curl http://api.mysite.com
```

### WordPress Site

```bash
# 1. Setup VPS
./bin/vpssetup setup
./bin/vpssetup harden

# 2. Install PHP-FPM (via SSH)
./bin/vpssetup connect
# On VPS:
sudo apt-get install php-fpm php-mysql php-curl php-gd php-mbstring php-xml php-zip
exit

# 3. Configure NGINX
./bin/vpssetup nginx setup
# Choose: php
# Domain: blog.mysite.com
# Include www: yes
# Path: /var/www/blog/public

# 4. Upload WordPress files
# (download WordPress, upload to VPS)

# 5. Create DNS
./bin/vpssetup dns create
# Type: A
# Name: blog
# Value: <vps-ip>

# 6. Complete WordPress installation via browser
```

## Troubleshooting

### Configuration Test Fails

```bash
# Test config
./bin/vpssetup nginx test

# If errors, check them manually
./bin/vpssetup connect
sudo nginx -t
```

Common issues:

- Missing PHP-FPM socket for PHP configs
- Duplicate server_name directives
- Syntax errors in custom configs

### 502 Bad Gateway (Reverse Proxy)

Causes:

- Application not running on specified port
- Firewall blocking localhost connections
- Application crashed

Debug:

```bash
./bin/vpssetup connect
# Check if app is running
sudo netstat -tlnp | grep :3000
# Check NGINX error log
sudo tail -f /var/log/nginx/error.log
```

### 403 Forbidden

Causes:

- Wrong file permissions
- No index file
- Directory listing disabled

Fix:

```bash
./bin/vpssetup connect
sudo chown -R www-data:www-data /var/www/yoursite
sudo chmod -R 755 /var/www/yoursite
```

## Advanced Usage

### Custom Configuration

You can modify the generated configs manually:

```bash
./bin/vpssetup connect
sudo nano /etc/nginx/sites-available/example.com
# Make changes
sudo nginx -t  # Test
sudo systemctl reload nginx  # Apply
```

### Multiple Sites

Run `nginx setup` multiple times for different domains:

```bash
./bin/vpssetup nginx setup  # First site
./bin/vpssetup nginx setup  # Second site
./bin/vpssetup nginx setup  # Third site
```

Each site gets its own configuration file.

### SSL with Let's Encrypt

After setting up NGINX:

```bash
./bin/vpssetup connect
# Install certbot
sudo apt-get install certbot python3-certbot-nginx
# Get certificate
sudo certbot --nginx -d example.com -d www.example.com
# Certbot automatically updates NGINX config
```

Then update your config to enable SSL:

```bash
./bin/vpssetup nginx setup
# Choose same domain
# Enable SSL: yes
```

## Next Steps

After NGINX configuration:

1. **Secure with SSL** - Use certbot or manually install certificates
2. **Monitor logs** - Check access and error logs regularly
3. **Optimize** - Add caching, CDN, or load balancing as needed
4. **Backup** - Keep backups of your NGINX configs

## Architecture

The NGINX implementation consists of:

1. **pkg/nginx/nginx.go** - Configuration generation with Go templates

   - Static site template
   - Reverse proxy template
   - PHP-FPM template

2. **cmd/handlers.go** - Command handlers

   - `runNginxSetup()` - Interactive setup wizard
   - `runNginxTest()` - Test configuration
   - `runNginxReload()` - Reload NGINX
   - Helper functions for installation and deployment

3. **cmd/main.go** - Command registration
   - `nginx` parent command
   - `nginx setup` subcommand
   - `nginx test` subcommand
   - `nginx reload` subcommand
