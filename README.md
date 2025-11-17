# VPSSetup - Automated VPS Deployment Tool

Fast and reliable VPS setup automation tool that eliminates the pain of manual server configuration. Automates VPS creation, DNS management, NGINX configuration, security hardening, and system updates.

**🌐 Now with Web Interface!** Manage your VPS infrastructure through a beautiful web UI in addition to the powerful CLI.

## Why?

Sick and tired of VPS configuration? The constant back-and-forth between different dashboards just to setup a VPS, then switching to SSH terminals, configuring NGINX, managing DNS records, setting up SSL certificates... It's exhausting and time-consuming.

Every single deployment meant:

- Logging into the VPS provider dashboard
- Spinning up a server
- Switching to the domain registrar to configure DNS
- SSH'ing into the server to install dependencies
- Writing NGINX configs from scratch
- Fighting with Let's Encrypt
- Going back to SSH to check if everything works
- Reading logs in the terminal to debug issues

**Enough is enough.** That's why I created this program - to automate all of this pain away and get back to actually building things instead of fighting with infrastructure.

## Features

### ✅ Implemented (All Phases Complete)

#### Phase 1 & 2: Infrastructure & Configuration

- **VPS Management** - Create, destroy, restart, status, connect to VPS
- **SSH Key Management** - Automatic SSH key generation and upload (with reuse detection)
- **Firewall Configuration** - Automatic security rules for SSH, HTTP, HTTPS
- **System Updates** - Automatic apt-get update & upgrade after VPS creation
- **Security Hardening** - Automated fail2ban, UFW firewall, and SSH hardening
- **DNS Management** - Full DNS record management (A, AAAA, CNAME, TXT, MX, NS, SRV) via sweb.ru API
- **NGINX Configuration** - Automatic NGINX setup with support for static sites, reverse proxy, and PHP
- **SSL Certificates** - Let's Encrypt integration for automatic HTTPS
- **Config Management** - Profile-based YAML configuration with VPS info persistence
- **Interactive CLI** - Beautiful, user-friendly prompts and progress indicators

#### Phase 3: Deployment & Monitoring

- **Deployment System** - Three deployment strategies:
  - **Docker Compose** - Full-stack apps with PostgreSQL, Redis, RabbitMQ
  - **Static Sites** - HTML/CSS/JS with automatic NGINX configuration
  - **Standalone** - Manual file upload for custom setups
- **SFTP Upload** - Secure file transfer with automatic exclusions (.git, node_modules, etc.)
- **Logs Management** - Fetch and view infrastructure logs:
  - NGINX access/error logs
  - System logs (syslog)
  - fail2ban logs
  - SSH authentication logs
  - Firewall (UFW) logs
  - Systemd service logs
- **Real Provider Data** - Fetch available regions and sizes from DigitalOcean API

#### Phase 4: Web Interface ✨ NEW!

- **REST API** - Complete HTTP API for all VPS operations
- **Web Dashboard** - Beautiful gradient UI for managing infrastructure
- **VPS Management UI** - Create, destroy, restart, harden VPS through browser
- **NGINX Configuration UI** - Setup static sites, reverse proxies, and PHP apps
- **SSL Management UI** - Install and renew Let's Encrypt certificates
- **Provider Integration** - View available regions and instance sizes
- **Real-time Status** - Live VPS status updates
- **Log Viewer** - View and filter system logs

### 🚀 Future Enhancements

- **Multiple Providers** - Support for AWS, Linode, Vultr, Hetzner
- **Rollback System** - Restore previous deployment versions
- **Health Monitoring** - Automated uptime checks and alerts
- **Backup Management** - Automated VPS snapshots and restore

## Prerequisites

- Go 1.24+ (with toolchain 1.24.1)
- DigitalOcean API token
- sweb.ru account for DNS management (optional)

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/Vova4o/VPSSetup.git
cd VPSSetup

# Build
go build -o bin/vpssetup ./cmd

# Or use make
make build
```

## Quick Start

### Option A: Web Interface 🌐

The easiest way to get started! Launch the web interface:

```bash
./bin/vpssetup web --port 8080
```

Then open your browser to http://localhost:8080

The web interface provides:

- 📊 **Dashboard** - VPS status and quick actions
- 🖥️ **VPS Management** - Create, restart, destroy, harden
- 🌐 **NGINX Setup** - Configure web servers (static, proxy, PHP)
- 🔒 **SSL Management** - Install and renew certificates
- 📝 **Logs Viewer** - View system and application logs

### Option B: CLI (Command Line) ⌨️

For power users and automation:

#### 1. Initialize Configuration

```bash
./bin/vpssetup init
```

This will:

- Ask for your DigitalOcean API token
- Optionally ask for DNS provider credentials (sweb.ru)
- Generate SSH keys (or use existing ones)
- Create a `config.yaml` file

### 2. Create a VPS

```bash
./bin/vpssetup setup
```

This will **interactively**:

- Let you choose region (e.g., Toronto, NYC, San Francisco)
- Let you choose instance size (e.g., $4/mo, $6/mo, $12/mo)
- Ask for VPS name
- Create the VPS with firewall configuration
- **Run system updates automatically** (apt-get update & upgrade)
- Save VPS info to config

### 3. Harden Security

```bash
./bin/vpssetup harden
```

This automatically:

- Installs and configures fail2ban
- Configures UFW firewall
- Hardens SSH configuration (disable root login, key-only auth)
- Sets up automatic security updates

### 4. Configure NGINX

```bash
./bin/vpssetup nginx setup
```

This interactively:

- Installs NGINX on the VPS
- Asks for domain name and configuration type (static/proxy/PHP)
- Generates optimized NGINX configuration
- Deploys and tests the configuration
- Creates document root directories
- Reloads NGINX with new settings

### 5. Manage DNS Records

```bash
# List all DNS records
./bin/vpssetup dns list

# Create a new DNS record (interactive)
./bin/vpssetup dns create

# Remove a DNS record (interactive)
./bin/vpssetup dns remove
```

Supported record types:

- **A** - IPv4 address
- **AAAA** - IPv6 address
- **CNAME** - Canonical name
- **TXT** - Text record
- **MX** - Mail exchange
- **NS** - Name server
- **SRV** - Service record

### 6. Connect to VPS

```bash
./bin/vpssetup connect
```

Opens SSH connection to your VPS using the configured SSH key.

### 7. Restart VPS

```bash
./bin/vpssetup restart
```

Reboots the VPS via DigitalOcean API.

## Command Reference

### Web Interface Commands

| Command | Description                           |
| ------- | ------------------------------------- |
| `web`   | Start web server (default port: 8080) |

Options:

- `--port` - Specify custom port (e.g., `--port 3000`)

### Infrastructure Commands

| Command   | Description                             |
| --------- | --------------------------------------- |
| `init`    | Initialize configuration and API tokens |
| `setup`   | Create and configure a new VPS          |
| `connect` | SSH into the VPS                        |
| `restart` | Reboot the VPS                          |
| `harden`  | Apply security hardening                |
| `status`  | Show VPS status and information         |
| `destroy` | Destroy and delete VPS instance         |

### NGINX Commands

| Command        | Description                            |
| -------------- | -------------------------------------- |
| `nginx setup`  | Interactive NGINX configuration wizard |
| `nginx test`   | Test NGINX configuration for errors    |
| `nginx reload` | Reload NGINX to apply changes          |

### DNS Commands

| Command      | Description             |
| ------------ | ----------------------- |
| `dns list`   | List all DNS records    |
| `dns create` | Create a new DNS record |
| `dns remove` | Delete a DNS record     |

### Deployment Commands

| Command       | Description                                              |
| ------------- | -------------------------------------------------------- |
| `upload`      | Interactive deployment wizard (Docker/Static/Standalone) |
| `ssl install` | Install Let's Encrypt SSL certificate                    |
| `ssl renew`   | Renew SSL certificate                                    |

### Logs Commands

| Command               | Description                  |
| --------------------- | ---------------------------- |
| `logs nginx`          | View NGINX access/error logs |
| `logs system`         | View system logs (syslog)    |
| `logs fail2ban`       | View fail2ban logs           |
| `logs ssh`            | View SSH authentication logs |
| `logs firewall`       | View UFW firewall logs       |
| `logs service <name>` | View systemd service logs    |

### Management Commands

| Command   | Description                            |
| --------- | -------------------------------------- |
| `destroy` | Remove VPS and optionally clean config |

## Global Flags

- `-p, --profile <name>` - Use specific profile
- `-c, --config <path>` - Config file path (default: `./config.yaml`)
- `-v, --verbose` - Verbose output
- `--dry-run` - Simulate actions without making changes

## Examples

### Complete Workflow

```bash
# 1. Initialize
./bin/vpssetup init

# 2. Create VPS
./bin/vpssetup setup
# Choose: Toronto, $6/mo droplet, name: "my-app"

# 3. Harden security
./bin/vpssetup harden

# 4. Configure NGINX
./bin/vpssetup nginx setup
# Type: proxy, Port: 3000, Domain: api.example.com

# 5. Configure DNS
./bin/vpssetup dns create
# Type: A, Name: api, Value: <your-vps-ip>

# 6. Connect to VPS
./bin/vpssetup connect
```

### NGINX Configuration Examples

#### Static Website

```bash
./bin/vpssetup nginx setup
# Choose: static
# Domain: example.com
# Include www: yes
# Root path: /var/www/example.com/html
# SSL: no (configure later with certbot)
```

#### Reverse Proxy for Node.js/Go App

```bash
./bin/vpssetup nginx setup
# Choose: proxy
# Domain: api.example.com
# Include www: no
# Port: 3000
# SSL: no (configure later)
```

#### PHP Application

```bash
./bin/vpssetup nginx setup
# Choose: php
# Domain: blog.example.com
# Include www: yes
# Root path: /var/www/blog/public
# SSL: no
```

### DNS Management Examples

```bash
# Create A record for subdomain
./bin/vpssetup dns create
# Choose: A, Enter: "api", Enter: "167.99.138.142"

# Create CNAME record
./bin/vpssetup dns create
# Choose: CNAME, Enter: "www", Enter: "example.com"

# Create TXT record for domain verification
./bin/vpssetup dns create
# Choose: TXT, Enter: "@", Enter: "google-site-verification=..."

# List all records
./bin/vpssetup dns list

# Remove a record (interactive selection)
./bin/vpssetup dns remove
```

## Configuration

The tool uses a `config.yaml` file:

```yaml
default_profile: default
profiles:
  default:
    vps:
      provider: digitalocean
      apitoken: your_do_token
      instanceid: "123456789"
      publicip: "192.0.2.1"
      name: my-vps
    ssh:
      keypath: ~/.ssh/vps_setup_key
      user: root
    domain:
      name: example.com
      dnsprovider: sweb.ru
      dnsapikey: your_sweb_token
```

## Troubleshooting

### SSH Connection Issues

If you can't connect to your VPS:

```bash
# Check VPS status
./bin/vpssetup status

# Verify SSH key
ls -la ~/.ssh/vps_setup_key*

# Try manual SSH connection
ssh -i ~/.ssh/vps_setup_key root@<vps-ip>
```

### DNS Not Working

- DNS propagation can take 5-15 minutes
- Verify record was created: `./bin/vpssetup dns list`
- Check with dig: `dig example.com`
- Ensure sweb.ru API token has proper permissions

### NGINX Configuration Errors

```bash
# Test NGINX configuration
./bin/vpssetup nginx test

# Check NGINX logs on VPS
ssh root@<vps-ip>
sudo tail -f /var/log/nginx/error.log

# Reload NGINX after fixes
./bin/vpssetup nginx reload
```

## Roadmap

- [x] Phase 1: VPS Infrastructure (Complete)
  - [x] VPS creation
  - [x] Firewall configuration
  - [x] SSH key management
  - [x] System updates
  - [x] Security hardening
  - [x] VPS destroy with config cleanup
- [x] Phase 2: DNS & NGINX Management (Complete)

  - [x] sweb.ru integration
  - [x] Full DNS record support (A, AAAA, CNAME, TXT, MX, NS, SRV)
  - [x] Interactive DNS commands
  - [x] NGINX configuration (static, proxy, PHP)
  - [x] NGINX test and reload commands

- [x] Phase 3: Application Deployment (Complete)

  - [x] SFTP file upload with smart exclusions
  - [x] SSL certificate automation (Let's Encrypt)
  - [x] Docker Compose deployment (PostgreSQL, Redis, RabbitMQ)
  - [x] Static site deployment with NGINX
  - [x] Standalone deployment
  - [x] Infrastructure log retrieval (nginx, system, fail2ban, ssh, firewall)
  - [x] Real provider API integration (regions, sizes)

- [ ] Phase 4: Production Features (Future)
  - [ ] Deployment rollback system
  - [ ] Multi-provider support (AWS, Linode, Vultr)
  - [ ] Health monitoring and alerts
  - [ ] Automated backups and restore
  - [ ] Load balancer configuration
  - [ ] Database migration tools

## License

MIT License

## Author

Created by Vova4o - [GitHub](https://github.com/Vova4o)
