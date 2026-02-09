# Contributors

## Maintainers

| Name | GitHub | Role |
|------|--------|------|
| Dyam Marcano | [@dyammarcano](https://github.com/dyammarcano) | Creator & Maintainer |

## Contributing

### Setup

1. Clone the repository
2. Install [Go 1.25+](https://go.dev/dl/)
3. Install [Task](https://taskfile.dev)
4. Install [golangci-lint](https://golangci-lint.run/welcome/install/)

### Development Workflow

```bash
# Run all quality checks
task check

# Run tests only
task test

# Run a single test
go test -v -run TestName ./...

# Format and lint
task fmt
task lint
```

### Code Conventions

- All errors wrapped with `fmt.Errorf("scout: action: %w", err)`
- Nil-safe receivers on Browser methods
- Cleanup functions returned from stateful operations (SetHeaders, EvalOnNewDocument)
- Tests use `newTestBrowser(t)` and `newTestServer()` from `testutil_test.go`
- Tests skip (not fail) when Chromium is unavailable
