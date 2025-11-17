# VPSSetup - AI Agent Documentation

This document provides a comprehensive guide for AI agents working with the VPSSetup codebase. It explains the architecture, module locations, and implementation details.

## Project Overview

**VPSSetup** is a CLI tool for automated VPS deployment and management. It handles everything from VPS provisioning to application deployment, with support for Docker Compose, static sites, and custom deployments.

**Language**: Go 1.24+  
**Framework**: Cobra CLI  
**Providers**: DigitalOcean (VPS), SWeb (DNS)

## Architecture Overview

```
VPSSetup/
├── cmd/                    # CLI commands and handlers
│   ├── main.go            # Entry point, Cobra root command
│   ├── handlers.go        # Command handlers (setup, deploy, logs, etc.)
│   └── web.go             # Web server command handler
├── api/                   # REST API (NEW!)
│   ├── server.go          # HTTP server, routing, middleware
│   ├── handlers_vps.go    # VPS management endpoints
│   ├── handlers_nginx_ssl.go  # NGINX and SSL endpoints
│   ├── handlers_simple.go # Config, provider data, stubs
│   └── utils.go           # API utilities
├── web/                   # Web UI (NEW!)
│   └── index.html         # Single-page application
├── internal/              # Internal packages
│   ├── config/            # Configuration management
│   ├── connection/        # SSH client wrapper
│   ├── deploy/            # Deployment strategies
│   ├── dns/               # DNS management
│   ├── hardening/         # Security hardening
│   ├── interactive/       # User interaction (prompts, spinners)
│   ├── logs/              # Log retrieval
│   ├── nginx/             # NGINX configuration
│   ├── setup/             # VPS setup orchestration
│   ├── ssl/               # Let's Encrypt SSL
│   └── upload/            # SFTP file upload
└── pkg/                   # Public packages
    ├── nginx/             # NGINX utilities
    ├── provider/          # VPS/DNS provider integrations
    └── utils/             # Utility functions
```

## Module Reference

### 1. VPS Management

**Location**: `pkg/provider/`, `internal/setup/`

**Implementation**:

- `pkg/provider/provider.go` - VPSProvider interface
- `pkg/provider/digitalocean.go` - DigitalOcean implementation
  - `CreateInstance()` - Create VPS with firewall
  - `DeleteInstance()` - Destroy VPS
  - `GetInstance()` - Get VPS info
  - `ListRegions()` - Fetch available regions from API
  - `ListSizes()` - Fetch available instance sizes from API
  - `ListImages()` - Fetch available OS images
- `internal/setup/setup.go` - Orchestrates VPS creation with updates

**Commands**:

- `vpssetup setup` - Create VPS (handler: `runSetup` in `cmd/handlers.go`)
- `vpssetup destroy` - Delete VPS (handler: `runDestroy`)
- `vpssetup status` - Show VPS info (handler: `runStatus`)
- `vpssetup restart` - Reboot VPS (handler: `runRestart`)

### 2. SSH Connection

**Location**: `internal/connection/`

**Implementation**:

- `connection.go` - SSHClient wrapper around golang.org/x/crypto/ssh
  - `Connect()` - Establish SSH connection
  - `ExecuteCommand()` - Run remote commands
  - `GetClient()` - Expose underlying \*ssh.Client for SFTP

**Usage**: All modules use SSHClient for remote operations

**Command**: `vpssetup connect` - Direct SSH (handler: `runConnect`)

### 3. DNS Management

**Location**: `internal/dns/`, `pkg/provider/sweb.go`

**Implementation**:

- `pkg/provider/sweb.go` - SWeb JSON-RPC API client
  - DNS record CRUD operations
  - Supports: A, AAAA, CNAME, TXT, MX, NS, SRV records
- `internal/dns/dns.go` - DNS service wrapper

**Commands**:

- `vpssetup dns list` - List records (handler: `runDNSList`)
- `vpssetup dns create` - Create record (handler: `runDNSCreate`)
- `vpssetup dns update` - Update record (handler: `runDNSUpdate`)
- `vpssetup dns remove` - Delete record (handler: `runDNSRemove`)

### 4. NGINX Configuration

**Location**: `internal/nginx/`, `pkg/nginx/`

**Implementation**:

- `internal/nginx/nginx.go` - NGINX service
  - `Install()` - Install nginx via apt
  - `Deploy()` - Deploy configuration
  - `Test()` - Test configuration
  - `Reload()` - Reload nginx
- Template generation in `cmd/handlers.go`:
  - `generateStaticConfig()` - Static site config
  - `generateProxyConfig()` - Reverse proxy config
  - `generatePHPConfig()` - PHP-FPM config

**Commands**:

- `vpssetup nginx setup` - Interactive wizard (handler: `runNginxSetup`)
- `vpssetup nginx test` - Test config (handler: `runNginxTest`)
- `vpssetup nginx reload` - Reload service (handler: `runNginxReload`)

### 5. SSL Certificates

**Location**: `internal/ssl/`

**Implementation**:

- `ssl.go` - Let's Encrypt integration
  - `Install()` - Install certbot and get certificate
  - `Renew()` - Renew certificate
  - Uses certbot with nginx plugin

**Commands**:

- `vpssetup ssl install` - Get certificate (handler: `runSSLInstall`)
- `vpssetup ssl renew` - Renew certificate (handler: `runSSLRenew`)

### 6. Security Hardening

**Location**: `internal/hardening/`

**Implementation**:

- `hardening.go` - Security automation
  - Install fail2ban
  - Configure UFW firewall
  - Harden SSH (disable root, password auth)
  - Setup auto-updates

**Command**: `vpssetup harden` (handler: `runHarden`)

### 7. Deployment System

**Location**: `internal/deploy/`, `internal/upload/`

**Implementation**:

- `internal/deploy/deployer.go` - Interfaces and types
  - `Deployer` interface: Deploy(), Status(), Rollback()
  - `DeploymentType`: docker-compose, static, standalone
  - `DeployOptions` struct with all config
- `internal/deploy/docker.go` - Docker Compose deployer
  - `installDocker()` - Install Docker CE
  - `installDockerCompose()` - Install docker-compose plugin
  - `generateDockerCompose()` - Generate docker-compose.yml
  - `generateServiceConfig()` - PostgreSQL/Redis/RabbitMQ configs
  - `generateEnvFile()` - Create .env with passwords
- `internal/deploy/static.go` - Static site and standalone deployers
  - `StaticDeployer.Deploy()` - Upload files, configure NGINX
  - `StandaloneDeployer.Deploy()` - Simple file upload
- `internal/upload/upload.go` - SFTP implementation
  - `NewService(*ssh.Client)` - Create SFTP client
  - `UploadProject()` - Upload directory with exclusions
  - `UploadFile()` - Single file upload
  - `DownloadFile()` - Download from VPS
  - Default exclusions: .git, node_modules, .env, **pycache**, etc.

**Commands**:

- `vpssetup upload` - Deployment wizard (handler: `runUpload`)
  - Calls: `runDockerComposeDeploy`, `runStaticDeploy`, or `runStandaloneDeploy`

**Deployment Flow**:

1. User runs `vpssetup upload`
2. Interactive wizard asks for deployment type
3. Type-specific questions (services, domain, paths)
4. SSH connection established
5. Files uploaded via SFTP
6. Configuration deployed (docker-compose.yml or NGINX)
7. Services started

### 8. Logs Management

**Location**: `internal/logs/`

**Implementation**:

- `logs.go` - LogService with remote log fetching
  - `GetNginxLogs()` - Access/error logs with --errors flag
  - `GetSystemLogs()` - Syslog via journalctl
  - `GetFail2BanLogs()` - fail2ban logs
  - `GetSSHLogs()` - SSH auth logs
  - `GetFirewallLogs()` - UFW logs
  - `GetServiceLogs()` - Systemd service logs
  - All support `-n` (lines) and `-g` (grep pattern) flags

**Commands**:

- `vpssetup logs nginx` (handler: `runLogsNginx`)
- `vpssetup logs system` (handler: `runLogsSystem`)
- `vpssetup logs fail2ban` (handler: `runLogsFail2Ban`)
- `vpssetup logs ssh` (handler: `runLogsSSH`)
- `vpssetup logs firewall` (handler: `runLogsFirewall`)
- `vpssetup logs service <name>` (handler: `runLogsService`)

### 9. Interactive CLI

**Location**: `internal/interactive/`

**Implementation**:

- `interactive.go` - User interaction utilities
  - `AskInput()` - Text input with default
  - `AskSelect()` - Selection from list
  - `ConfirmAction()` - Yes/no confirmation
  - `ShowSpinner()` - Loading indicator
  - `Success()`, `Error()`, `Info()`, `Warning()` - Colored output
  - `getRegionsForProvider()` - Fetch regions from provider API
  - `getSizesForProvider()` - Fetch sizes from provider API

**Used by**: All command handlers for user interaction

### 10. Configuration

**Location**: `internal/config/`

**Implementation**:

- `config.go` - YAML configuration management
  - `Config` struct - Top-level config
  - `Profile` struct - VPS profile
  - `Load()` - Read config.yaml
  - `Save()` - Write config.yaml
  - `Validate()` - Validate config structure
  - `Profile.Validate()` - Validate profile settings

**Config Structure**:

```yaml
default_profile: default
profiles:
  default:
    vps:
      provider: digitalocean
      apikey: "..."
      region: nyc3
      size: s-1vcpu-1gb
      instanceid: "123456"
      publicip: "192.0.2.1"
    ssh:
      keypath: ~/.ssh/vps_key
      user: root
    domain:
      name: example.com
      dnsprovider: sweb
      dnsapikey: "..."
    project:
      path: ./app
      port: 3000
      runtime: node
    ssl:
      email: admin@example.com
      enable: true
      autorenew: true
```

## Key Design Patterns

### 1. Provider Pattern

All VPS providers implement the `VPSProvider` interface for consistent operations across different cloud platforms.

### 2. Strategy Pattern

Deployment uses different strategies (Docker, Static, Standalone) implementing the `Deployer` interface.

### 3. Service Wrapper Pattern

Each major feature (DNS, SSL, NGINX, Logs) is wrapped in a service struct that handles SSH operations.

### 4. Interactive Wizard Pattern

Complex operations use step-by-step wizards with `internal/interactive` utilities.

## Common Tasks for AI Agents

### Adding a New VPS Provider

1. Create `pkg/provider/newprovider.go`
2. Implement `VPSProvider` interface
3. Add case in `cmd/handlers.go` provider selection
4. Update `internal/interactive/interactive.go` region/size fetchers

### Adding a New Deployment Type

1. Create deployer in `internal/deploy/newtype.go`
2. Implement `Deployer` interface
3. Add new `DeploymentType` constant
4. Add handler function in `cmd/handlers.go`
5. Update `runUpload` wizard with new option

### Adding a New Log Source

1. Add method to `internal/logs/logs.go`
2. Create command in `cmd/main.go`
3. Add handler in `cmd/handlers.go`
4. Follow existing pattern (support -n, -g flags)

### Adding a New Command

1. Define command in `cmd/main.go` using Cobra
2. Create handler function in `cmd/handlers.go`
3. Use `internal/interactive` for user prompts
4. Use `internal/connection` for SSH operations
5. Update README.md command reference

## Dependencies

**Core**:

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `golang.org/x/crypto/ssh` - SSH client
- `github.com/pkg/sftp` - SFTP file transfer

**Provider APIs**:

- `github.com/digitalocean/godo` - DigitalOcean API
- Custom JSON-RPC for SWeb

**UI**:

- `github.com/AlecAivazis/survey/v2` - Interactive prompts
- `github.com/briandowns/spinner` - Loading spinners

## Testing

**Location**: `tests/`

**Current State**: Minimal test coverage

**Test Files**:

- `cmd_test.go` - Command tests
- `interactive_test.go` - Interactive utility tests

**To Add Tests**:

1. Create `*_test.go` file in same package
2. Use standard Go testing
3. Mock SSH/API calls where needed

## Known Limitations & Future Work

### Current Limitations

1. **Rollback**: Not implemented - all Rollback() methods return error
2. **Multi-provider**: Only DigitalOcean fully supported for VPS
3. **DNS Provider**: Only SWeb supported
4. **Tests**: Minimal coverage

### Future Enhancements

1. Implement rollback with version tracking
2. Add AWS, Linode, Vultr, Hetzner support
3. Add Cloudflare, Route53 DNS support
4. Health monitoring system
5. Automated backup/restore
6. Load balancer configuration

## Build & Development

```bash
# Build
go build -o bin/vpssetup cmd/*.go

# Or use Makefile
make build

# Run
./bin/vpssetup <command>

# With flags
./bin/vpssetup setup --verbose --dry-run
```

## Error Handling

- All functions return `error` as last return value
- Use `fmt.Errorf()` with `%w` for error wrapping
- Interactive handlers show errors with `interactive.Error()`
- Validate inputs early before making API calls

## Logging & Output

- Use `interactive.Info()`, `Success()`, `Error()`, `Warning()` for colored output
- Use `interactive.ShowSpinner()` for long operations
- `--verbose` flag enables detailed output
- `--dry-run` flag simulates actions without changes

## Web Interface (NEW!)

### REST API Architecture

**Location**: `api/`

**Server Setup** (`api/server.go`):

- Gorilla mux router for HTTP routing
- CORS-enabled WebSocket upgrader
- JSON response helpers
- Timeout contexts for operations

**Endpoints**:

**VPS Management** (`api/handlers_vps.go`):

- `POST /api/vps/setup` - Create VPS (reads SSH public key from `~/.ssh/vpssetup_rsa.pub`)
- `GET /api/vps/status` - Get VPS status
- `POST /api/vps/destroy` - Delete VPS
- `POST /api/vps/restart` - Reboot VPS
- `GET /api/vps/connect` - Get SSH connection info
- `POST /api/vps/harden` - Apply security hardening

**NGINX & SSL** (`api/handlers_nginx_ssl.go`):

- `POST /api/nginx/setup` - Configure NGINX (static/proxy/PHP)
- `POST /api/nginx/test` - Test configuration
- `POST /api/nginx/reload` - Reload NGINX
- `POST /api/ssl/install` - Install Let's Encrypt certificate
- `POST /api/ssl/renew` - Renew certificate

**Provider Data** (`api/handlers_simple.go`):

- `GET /api/providers/regions` - List available regions
- `GET /api/providers/sizes` - List instance sizes
- `GET /api/config` - Get current config
- `PUT /api/config` - Update config

**Important**: SSH Adapter Pattern
The API uses an adapter to bridge `SSHClient.ExecuteCommand()` to `nginx.SSHExecutor.RunCommand()`:

```go
type sshAdapter struct {
    client *connection.SSHClient
}

func (a *sshAdapter) RunCommand(cmd string) (string, error) {
    return a.client.ExecuteCommand(cmd)
}
```

### Web UI

**Location**: `web/index.html`

- Single-page application with tabs:
  - Dashboard - VPS overview
  - VPS - Create/manage VPS
  - DNS - DNS records (CLI-recommended)
  - NGINX - Web server configuration
  - Deploy - Application deployment (CLI-recommended)
  - Logs - System logs viewer
- Gradient design with responsive layout
- Real-time status updates
- Dynamic region/size dropdowns from API

**Command**: `vpssetup web --port 8080`

### API Key Features

1. **SSH Key Authentication**: Automatically reads public key from `.pub` file
2. **Timeout Management**: Context timeouts for long operations
3. **Error Handling**: Detailed error messages with nil-safety
4. **Config Persistence**: Updates saved after VPS creation
5. **Apt Lock Retry**: Waits up to 5 minutes for system package locks

### Development Notes

**Adding New Endpoints**:

1. Add handler function in appropriate `handlers_*.go` file
2. Register route in `server.go` `setupRoutes()`
3. Use `respondSuccess()` and `respondError()` for consistent responses
4. Add timeout context for long-running operations

**Testing API**: Use curl or the web UI at http://localhost:8080

## Configuration Management

- Always load config with `config.Load(configPath)`
- Get profile with `cfg.GetProfile(profileName)`
- Validate with `profile.Validate()` before operations
- Save updates with `cfg.Save(configPath)`
- Store VPS info after creation for reuse
- **Web API**: Config is loaded once at server startup in `api.NewServer()`

## SSH Best Practices

1. Always use `defer sshClient.Close()`
2. **Web API**: Read public key from `keyPath + ".pub"` for VPS setup
3. Use `utils.ExpandPath()` to handle `~` in paths
4. Check connection with `sshClient.Connect()`
5. Use `ExecuteCommand()` for single commands
6. Use `GetClient()` for SFTP operations
7. Expand paths with `expandPath()` for ~ support

## This is the Complete Reference

All major features are implemented. This document provides the architectural map for AI agents to understand and modify the codebase efficiently.
