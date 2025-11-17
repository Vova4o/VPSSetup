package logs

import (
	"fmt"
	"strings"
)

// SSHExecutor defines the interface for SSH command execution
type SSHExecutor interface {
	RunCommand(cmd string) (string, error)
}

// Service handles log operations
type Service struct {
	sshClient SSHExecutor
}

// NewService creates a new logs service
func NewService(sshClient SSHExecutor) *Service {
	return &Service{
		sshClient: sshClient,
	}
}

// LogOptions configures log retrieval
type LogOptions struct {
	Lines  int    // Number of lines to retrieve
	Grep   string // Pattern to search for
	Follow bool   // Stream logs in real-time (not yet implemented for SSH)
}

// GetNginxLogs retrieves NGINX access or error logs
func (s *Service) GetNginxLogs(domain string, logType string, opts LogOptions) (string, error) {
	var logPath string

	if logType == "error" {
		logPath = fmt.Sprintf("/var/log/nginx/%s.error.log", domain)
	} else {
		logPath = fmt.Sprintf("/var/log/nginx/%s.access.log", domain)
	}

	// If no domain specified, use main nginx logs
	if domain == "" {
		if logType == "error" {
			logPath = "/var/log/nginx/error.log"
		} else {
			logPath = "/var/log/nginx/access.log"
		}
	}

	return s.getLogs(logPath, opts)
}

// GetSystemLogs retrieves system logs via journalctl
func (s *Service) GetSystemLogs(opts LogOptions) (string, error) {
	cmd := fmt.Sprintf("sudo journalctl -n %d --no-pager", opts.Lines)

	if opts.Grep != "" {
		cmd = fmt.Sprintf("%s | grep -i '%s'", cmd, opts.Grep)
	}

	output, err := s.sshClient.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve system logs: %w", err)
	}

	return output, nil
}

// GetFail2banLogs retrieves fail2ban logs
func (s *Service) GetFail2banLogs(opts LogOptions) (string, error) {
	return s.getLogs("/var/log/fail2ban.log", opts)
}

// GetSSHLogs retrieves SSH authentication logs
func (s *Service) GetSSHLogs(opts LogOptions) (string, error) {
	// SSH logs are in auth.log on Ubuntu/Debian
	return s.getLogs("/var/log/auth.log", opts)
}

// GetFirewallLogs retrieves UFW firewall logs
func (s *Service) GetFirewallLogs(opts LogOptions) (string, error) {
	// UFW logs to syslog, so we need to grep for UFW entries
	cmd := fmt.Sprintf("sudo grep -i ufw /var/log/syslog | tail -n %d", opts.Lines)

	if opts.Grep != "" {
		cmd = fmt.Sprintf("%s | grep -i '%s'", cmd, opts.Grep)
	}

	output, err := s.sshClient.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve firewall logs: %w", err)
	}

	return output, nil
}

// GetServiceLogs retrieves logs for a specific systemd service
func (s *Service) GetServiceLogs(serviceName string, opts LogOptions) (string, error) {
	cmd := fmt.Sprintf("sudo journalctl -u %s -n %d --no-pager", serviceName, opts.Lines)

	if opts.Grep != "" {
		cmd = fmt.Sprintf("%s | grep -i '%s'", cmd, opts.Grep)
	}

	output, err := s.sshClient.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve service logs for %s: %w", serviceName, err)
	}

	return output, nil
}

// getLogs is a helper function to retrieve logs from a file
func (s *Service) getLogs(path string, opts LogOptions) (string, error) {
	// Check if file exists first
	checkCmd := fmt.Sprintf("sudo test -f %s && echo 'exists' || echo 'not found'", path)
	checkOutput, err := s.sshClient.RunCommand(checkCmd)
	if err == nil && strings.TrimSpace(checkOutput) == "not found" {
		return "", fmt.Errorf("log file not found: %s", path)
	}

	// Build tail command
	cmd := fmt.Sprintf("sudo tail -n %d %s", opts.Lines, path)

	// Add grep if pattern specified
	if opts.Grep != "" {
		// Escape single quotes in grep pattern
		escapedPattern := strings.ReplaceAll(opts.Grep, "'", "'\\''")
		cmd = fmt.Sprintf("%s | grep -i '%s'", cmd, escapedPattern)
	}

	output, err := s.sshClient.RunCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve logs from %s: %w", path, err)
	}

	return output, nil
}

// ListAvailableNginxLogs lists all available nginx log files
func (s *Service) ListAvailableNginxLogs() ([]string, error) {
	cmd := "sudo ls -1 /var/log/nginx/*.log 2>/dev/null || echo ''"
	output, err := s.sshClient.RunCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list nginx logs: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return []string{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	return lines, nil
}
