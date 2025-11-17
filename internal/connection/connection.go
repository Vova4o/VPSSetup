package connection

import (
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages SSH connections to VPS
type SSHClient struct {
	client *ssh.Client
	config *ssh.ClientConfig
	host   string
}

// NewSSHClient creates a new SSH client
func NewSSHClient(host, user, keyPath string) (*SSHClient, error) {
	// Load private key
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	// Create SSH config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	return &SSHClient{
		config: config,
		host:   net.JoinHostPort(host, "22"),
	}, nil
}

// Connect establishes SSH connection
func (c *SSHClient) Connect() error {
	client, err := ssh.Dial("tcp", c.host, c.config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	c.client = client
	return nil
}

// ExecuteCommand runs a command on the remote server
func (c *SSHClient) ExecuteCommand(cmd string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// HealthCheck performs connection health check
func (c *SSHClient) HealthCheck() error {
	_, err := c.ExecuteCommand("echo ok")
	return err
}

// WaitForReady waits for SSH to be available
func (c *SSHClient) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 2 * time.Second

	for time.Now().Before(deadline) {
		err := c.Connect()
		if err == nil {
			if err := c.HealthCheck(); err == nil {
				return nil
			}
			c.Close()
		}

		time.Sleep(backoff)
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}

	return fmt.Errorf("timeout waiting for SSH to be ready")
}

// GetClient returns the underlying SSH client
func (c *SSHClient) GetClient() *ssh.Client {
	return c.client
}
