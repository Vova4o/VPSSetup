package deploy

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Vova4o/VPSSetup/internal/connection"
)

// DeploymentType represents the type of deployment
type DeploymentType string

const (
	DeploymentTypeDockerCompose DeploymentType = "docker-compose"
	DeploymentTypeStatic        DeploymentType = "static"
	DeploymentTypeStandalone    DeploymentType = "standalone"
)

// Deployer interface for different deployment strategies
type Deployer interface {
	Deploy(ctx context.Context, opts DeployOptions) error
	Status(ctx context.Context) (*DeployStatus, error)
	// Rollback rolls back to previous deployment version
	// Note: Not currently implemented - reserved for future use
	Rollback(ctx context.Context) error
}

// DeployOptions contains deployment configuration
type DeployOptions struct {
	Type       DeploymentType
	LocalPath  string
	RemotePath string
	SSH        *connection.SSHClient
	Domain     string
	Port       int
	Env        map[string]string

	// Docker Compose specific
	ComposeFile string
	Services    []Service

	// Static site specific
	NginxConfig string
}

// Service represents a service in docker-compose
type Service struct {
	Name    string
	Type    string // postgres, redis, rabbitmq, custom
	Version string
	Port    int
	Env     map[string]string
	Volumes []string
}

// DeployStatus represents deployment status
type DeployStatus struct {
	Healthy  bool
	Services map[string]string
	Message  string
}

// NewDeployer creates appropriate deployer based on type
func NewDeployer(deployType DeploymentType) (Deployer, error) {
	switch deployType {
	case DeploymentTypeDockerCompose:
		return &DockerComposeDeployer{}, nil
	case DeploymentTypeStatic:
		return &StaticDeployer{}, nil
	case DeploymentTypeStandalone:
		return &StandaloneDeployer{}, nil
	default:
		return nil, fmt.Errorf("unsupported deployment type: %s", deployType)
	}
}

// GeneratePassword generates a random password
func GeneratePassword() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	const length = 16

	password := make([]byte, length)
	for i := range password {
		password[i] = charset[rand.Intn(len(charset))]
	}

	return string(password)
}
