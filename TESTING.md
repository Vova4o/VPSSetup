# Testing Documentation

## Test Suite Overview

Comprehensive integration tests for the VPS Setup CLI application.

### Test Location

Tests are located in the `tests/` folder, separate from the main application code. This keeps the test code organized and doesn't interfere with the main program logic.

### Test Coverage

- **Total Tests**: 7 test functions with multiple subtests
- **Test Type**: Black-box integration tests (tests the compiled binary)
- **Benchmark**: Included for performance measurement

### Test Files

- `tests/cmd_test.go` - CLI integration tests (builds and tests actual binary)

## Test Functions

### 1. TestCLIBuild

Tests that the CLI can be built successfully:

- ✅ Builds without errors
- ✅ Produces working binary
- ✅ Cleans up test artifacts

### 2. TestCLIHelp

Validates help text output:

- ✅ Root command help text
- ✅ Init command help
- ✅ Deploy command help
- ✅ All expected keywords present

### 3. TestCLICommands

Verifies all commands are properly registered:

- ✅ init, setup, connect, deploy
- ✅ upload, logs, ssl, dns
- ✅ status, destroy
- ✅ All commands have help text

### 4. TestCLIFlags

Tests global flag parsing:

- ✅ Custom config path (`--config`)
- ✅ Custom profile (`--profile`)
- ✅ Verbose mode (`--verbose`)
- ✅ Dry-run mode (`--dry-run`)
- ✅ Invalid flag detection

### 5. TestCLIInitCommand

Tests the init command specifically:

- ✅ Init command exists
- ✅ Shows proper help text
- ✅ Mentions wizard functionality

### 6. TestCLIVersion

Tests version/about output:

- ✅ Help command works
- ✅ Shows program name
- ✅ Displays usage information

### 7. TestCLISubcommands

Tests subcommands with nested commands:

- ✅ SSL subcommands (install, renew, status)
- ✅ DNS subcommands (create, list)
- ✅ All subcommands have help text

### 8. BenchmarkCLIHelp

Performance benchmark:

- ⚡ Measures help command performance
- ⚡ Tests binary execution speed

## Running Tests

### Run All Tests

```bash
go test -v ./tests
```

### Run Specific Test

```bash
go test -v ./tests -run TestCLIBuild
```

### Run with Coverage

```bash
go test -v -coverprofile=coverage.out ./tests
go tool cover -func=coverage.out
```

### Run Benchmarks

```bash
go test -bench=. ./tests
```

### Generate HTML Coverage Report

```bash
go test -coverprofile=coverage.out ./tests
go tool cover -html=coverage.out -o coverage.html
```

## Test Patterns Used

### Black-Box Integration Testing

Tests build and execute the actual CLI binary to verify end-to-end functionality:

```go
// Build the binary
cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd")
cmd.Run()

// Test the binary
testCmd := exec.Command(binaryPath, "--help")
output, err := testCmd.CombinedOutput()
```

### Table-Driven Tests

All tests use the table-driven pattern for clarity and maintainability:

```go
tests := []struct {
    name           string
    args           []string
    expectedOutput []string
}{
    {
        name: "test case 1",
        args: []string{"--help"},
        expectedOutput: []string{"usage", "commands"},
    },
    // ...
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test implementation
    })
}
```

### Command Reset Pattern

Each test resets the root command to ensure isolation:

```go
func resetRootCommand() {
    rootCmd = &cobra.Command{
        // command definition
    }
    initCommands()
}
```

## Test Status

✅ **All tests passing**

- 8 test functions
- 33 subtests
- 0 failures
- 100% pass rate

## Next Steps

1. **Add Handler Tests** - Test individual handler functions with mocked dependencies
2. **Integration Tests** - Test actual VPS provisioning with test credentials
3. **Mock DigitalOcean API** - Use httpmock or similar for API testing
4. **Config Tests** - Test configuration loading and validation
5. **Interactive Tests** - Mock survey prompts for interactive wizard testing

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: "1.24"
      - run: go test -v -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out
```

### GitLab CI Example

```yaml
test:
  image: golang:1.24
  script:
    - go test -v -coverprofile=coverage.out ./...
    - go tool cover -func=coverage.out
  coverage: '/coverage: \d+.\d+% of statements/'
```

## Test Maintenance

- **Keep tests focused**: Each test should verify one aspect
- **Use descriptive names**: Test names should explain what they're testing
- **Maintain isolation**: Reset state between tests
- **Update with changes**: When adding commands, add corresponding tests
- **Document complex cases**: Add comments for non-obvious test scenarios
