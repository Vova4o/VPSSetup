package tests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIBuild tests that the CLI can be built successfully
func TestCLIBuild(t *testing.T) {
	projectRoot := getProjectRoot(t)

	cmd := exec.Command("go", "build", "-o", filepath.Join(projectRoot, "bin", "vpssetup-test"), filepath.Join(projectRoot, "cmd"))
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, output)
	}

	// Clean up test binary
	defer os.Remove(filepath.Join(projectRoot, "bin", "vpssetup-test"))
}

// TestCLIHelp tests the help command output
func TestCLIHelp(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	tests := []struct {
		name           string
		args           []string
		expectedOutput []string
	}{
		{
			name: "root help",
			args: []string{"--help"},
			expectedOutput: []string{
				"fast and reliable VPS setup automation",
				"init",
				"deploy",
				"setup",
			},
		},
		{
			name: "init help",
			args: []string{"init", "--help"},
			expectedOutput: []string{
				"init",
				"wizard",
			},
		},
		{
			name: "deploy help",
			args: []string{"deploy", "--help"},
			expectedOutput: []string{
				"deploy",
				"deployment pipeline",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()
			// Help command exits with 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					t.Logf("Command exited with code %d (expected for some help commands)", exitErr.ExitCode())
				}
			}

			outputStr := string(output)
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, got:\n%s", expected, outputStr)
				}
			}
		})
	}
}

// TestCLICommands tests that all expected commands exist
func TestCLICommands(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	commands := []string{
		"init",
		"setup",
		"connect",
		"deploy",
		"upload",
		"logs",
		"ssl",
		"dns",
		"status",
		"destroy",
	}

	for _, command := range commands {
		t.Run("command_"+command, func(t *testing.T) {
			cmd := exec.Command(binary, command, "--help")
			output, err := cmd.CombinedOutput()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					// Help command might exit with non-zero
					t.Logf("Command exited with code %d", exitErr.ExitCode())
				}
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, command) {
				t.Errorf("Command %q help should contain command name, got:\n%s", command, outputStr)
			}
		})
	}
}

// TestCLIFlags tests global flag parsing
func TestCLIFlags(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	tests := []struct {
		name        string
		args        []string
		shouldError bool
	}{
		{
			name:        "config flag",
			args:        []string{"--config", "/tmp/test.yaml", "--help"},
			shouldError: false,
		},
		{
			name:        "profile flag",
			args:        []string{"--profile", "staging", "--help"},
			shouldError: false,
		},
		{
			name:        "verbose flag",
			args:        []string{"--verbose", "--help"},
			shouldError: false,
		},
		{
			name:        "dry-run flag",
			args:        []string{"--dry-run", "--help"},
			shouldError: false,
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for invalid flag, got none. Output: %s", output)
				}
			} else {
				// Help commands might exit with non-zero but shouldn't fail parsing
				outputStr := string(output)
				if strings.Contains(outputStr, "unknown flag") {
					t.Errorf("Flag parsing failed: %s", outputStr)
				}
			}
		})
	}
}

// TestCLIInitCommand tests the init command specifically
func TestCLIInitCommand(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	// Test that init command exists and shows help
	cmd := exec.Command(binary, "init", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			t.Logf("Init help exited with code %d", exitErr.ExitCode())
		}
	}

	outputStr := string(output)
	expectedTexts := []string{"init", "wizard"}

	for _, expected := range expectedTexts {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Init help should contain %q, got:\n%s", expected, outputStr)
		}
	}
}

// TestCLIVersion tests version/about output
func TestCLIVersion(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	// Most CLIs show usage when --version is not defined
	cmd := exec.Command(binary, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			t.Logf("Help exited with code %d", exitErr.ExitCode())
		}
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "vpssetup") {
		t.Errorf("Help should mention vpssetup, got:\n%s", outputStr)
	}
}

// TestCLISubcommands tests that subcommands with subcommands work
func TestCLISubcommands(t *testing.T) {
	projectRoot := getProjectRoot(t)
	binary := buildTestBinary(t, projectRoot)
	defer os.Remove(binary)

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			name:           "ssl install",
			args:           []string{"ssl", "install", "--help"},
			expectedOutput: "install",
		},
		{
			name:           "ssl renew",
			args:           []string{"ssl", "renew", "--help"},
			expectedOutput: "renew",
		},
		{
			name:           "ssl status",
			args:           []string{"ssl", "status", "--help"},
			expectedOutput: "status",
		},
		{
			name:           "dns create",
			args:           []string{"dns", "create", "--help"},
			expectedOutput: "create",
		},
		{
			name:           "dns list",
			args:           []string{"dns", "list", "--help"},
			expectedOutput: "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binary, tt.args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					t.Logf("Subcommand help exited with code %d", exitErr.ExitCode())
				}
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.expectedOutput, outputStr)
			}
		})
	}
}

// BenchmarkCLIHelp benchmarks the help command performance
func BenchmarkCLIHelp(b *testing.B) {
	projectRoot := getProjectRoot(&testing.T{})
	binary := buildTestBinary(&testing.T{}, projectRoot)
	defer os.Remove(binary)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binary, "--help")
		_ = cmd.Run()
	}
}

// Helper Functions

func getProjectRoot(t *testing.T) string {
	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// If we're in tests directory, go up one level
	if strings.HasSuffix(cwd, "tests") {
		return filepath.Dir(cwd)
	}

	return cwd
}

func buildTestBinary(t *testing.T, projectRoot string) string {
	binaryPath := filepath.Join(projectRoot, "bin", "vpssetup-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(projectRoot, "cmd"))
	cmd.Dir = projectRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test binary: %v\nStderr: %s", err, stderr.String())
	}

	return binaryPath
}

// TestMain for setup/teardown
func TestMain(m *testing.M) {
	// Setup
	code := m.Run()
	// Teardown
	os.Exit(code)
}
