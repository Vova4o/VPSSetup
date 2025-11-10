# VPS Setup Automation

Fast and reliable VPS setup automation tool for quick deployment of web projects with subdomain registration, NGINX configuration, and SSL certificate setup.

## Why?

I'm sick and tired of VPS configuration. The constant back-and-forth between different dashboards just to setup a VPS, then switching to SSH terminals to read logs, configure NGINX, setup SSL certificates, manage DNS records... It's exhausting and time-consuming.

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

## Overview

This project streamlines the VPS setup process, eliminating hours of manual terminal configuration. Perfect for testing projects or quickly deploying websites with proper domain configuration, web server setup, and SSL encryption.

## Features

- 🚀 **Fast Setup** - Automated VPS provisioning and configuration
- 🌐 **Subdomain Registration** - Automatic DNS configuration
- 📦 **Project Upload** - Seamless code deployment to VPS
- 🔒 **SSL/TLS** - Automatic SSL certificate generation and renewal
- ⚙️ **NGINX Configuration** - Auto-configured reverse proxy and web server
- 📊 **Log Management** - Easy access to VPS logs and data

## Use Cases

- Quick testing and staging environments
- Rapid website deployment
- Development environment setup
- Small to medium project hosting

## Prerequisites

- Go 1.19+ installed
- SSH key pair for VPS access
- VPS provider account (DigitalOcean, Linode, Vultr, etc.)
- Domain name with DNS access
- API credentials for your VPS provider

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd VPSSetup

# Build the project
go build -o vpssetup

# Make executable
chmod +x vpssetup
```

## Configuration

Create a `config.yaml` file in the project root:

```yaml
vps:
  provider: "digitalocean" # or linode, vultr, etc.
  api_key: "your-api-key"
  region: "nyc3"
  size: "s-1vcpu-1gb"

domain:
  name: "example.com"
  subdomain: "app"
  dns_provider: "cloudflare"
  dns_api_key: "your-dns-api-key"

project:
  path: "./your-project"
  port: 8080

ssl:
  email: "your-email@example.com"
  enable: true
```

## Usage

### Quick Start

```bash
# Run complete setup
./vpssetup deploy --config config.yaml

# Or use individual commands for specific tasks
./vpssetup setup --config config.yaml
./vpssetup upload --config config.yaml
```

### Available Commands

```bash
# 1. Initial VPS setup
./vpssetup setup --config config.yaml

# 2. Create and connect VPS
./vpssetup connect --config config.yaml

# 3. Upload project and configure VPS
./vpssetup upload --config config.yaml

# 4. Get logs from VPS
./vpssetup logs --tail 100

# 5. Get data/files from VPS
./vpssetup download --remote /path/to/file --local ./downloads/
```

## Project Structure

```
VPSSetup/
├── cmd/
│   └── main.go              # CLI entry point
├── internal/
│   ├── setup/               # VPS setup logic
│   ├── connection/          # VPS connection handling
│   ├── upload/              # Project upload and deployment
│   ├── logs/                # Log retrieval and management
│   ├── nginx/               # NGINX configuration
│   ├── ssl/                 # SSL/Let's Encrypt integration
│   └── dns/                 # DNS/subdomain management
├── pkg/
│   ├── provider/            # VPS provider interfaces
│   └── utils/               # Utility functions
├── config.yaml              # Configuration file
├── README.md
└── go.mod
```

## Implementation TODO List

### Phase 1: Setup

- [ ] Design CLI interface and command structure
- [ ] Implement configuration file parser (YAML/JSON)
- [ ] Create VPS provider abstraction layer
- [ ] Add support for multiple VPS providers (DigitalOcean, Linode, Vultr)
- [ ] Implement VPS instance creation
- [ ] Setup SSH key management
- [ ] Add firewall rules configuration (ports 80, 443, 22)
- [ ] Implement system updates and basic security hardening

### Phase 2: Connection and Creation

- [ ] Implement SSH connection management
- [ ] Add connection pooling and retry logic
- [ ] Create health check mechanism
- [ ] Implement VPS status monitoring
- [ ] Add connection validation and diagnostics
- [ ] Setup SSH config file generation
- [ ] Implement non-interactive authentication
- [ ] Add connection timeout handling

### Phase 3: Upload and Setup VPS

- [ ] Implement secure file transfer (SFTP/SCP)
- [ ] Create project directory structure on VPS
- [ ] Add dependency installation (language-specific)
- [ ] Implement NGINX configuration generation
- [ ] Setup NGINX reverse proxy rules
- [ ] Integrate Let's Encrypt for SSL certificates
- [ ] Configure automatic SSL renewal
- [ ] Setup DNS records via provider API
- [ ] Implement subdomain creation and validation
- [ ] Add application service management (systemd)
- [ ] Create startup scripts for application
- [ ] Implement health checks after deployment
- [ ] Add rollback mechanism for failed deployments

### Phase 4: Get Logs and Data from VPS

- [ ] Implement remote log retrieval
- [ ] Add log streaming capability
- [ ] Create log filtering and searching
- [ ] Implement file download from VPS
- [ ] Add backup creation and download
- [ ] Setup monitoring metrics collection
- [ ] Create dashboard for VPS status
- [ ] Implement alert system for errors
- [ ] Add database backup functionality
- [ ] Create scheduled backup mechanism

### Additional Features (Future)

- [ ] Multi-server deployment support
- [ ] Load balancer configuration
- [ ] Docker container deployment option
- [ ] Kubernetes integration
- [ ] CI/CD pipeline integration
- [ ] Zero-downtime deployment
- [ ] A/B testing support
- [ ] Environment variable management
- [ ] Secrets management integration
- [ ] Cost estimation and tracking
- [ ] Auto-scaling configuration
- [ ] Database provisioning and migration

## Examples

### Deploy a Go Web Application

```bash
./vpssetup deploy \
  --provider digitalocean \
  --region nyc3 \
  --domain myapp.example.com \
  --project ./my-go-app \
  --port 8080
```

### Deploy a Node.js Application

```bash
./vpssetup deploy \
  --provider linode \
  --region us-east \
  --domain api.example.com \
  --project ./my-node-app \
  --port 3000 \
  --runtime nodejs
```

## Troubleshooting

### Connection Issues

```bash
# Test SSH connection
./vpssetup test-connection --config config.yaml

# Verify SSH key
./vpssetup verify-ssh
```

### SSL Certificate Issues

```bash
# Force SSL renewal
./vpssetup ssl-renew --force

# Check SSL status
./vpssetup ssl-status
```

### Logs Not Accessible

```bash
# Check VPS status
./vpssetup status

# Test connection and permissions
./vpssetup diagnose
```

## Security Considerations

- SSH keys are used exclusively (no password authentication)
- Automatic security updates can be enabled
- Firewall rules restrict access to necessary ports only
- SSL/TLS encryption for all web traffic
- Secrets and API keys should be stored securely (use environment variables)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - See LICENSE file for details

## Support

For issues, questions, or contributions, please open an issue on GitHub.

---

**Note**: This project is designed for quick deployments and testing. For production environments, consider additional security hardening and monitoring solutions.
