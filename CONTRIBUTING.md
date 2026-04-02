# Contributing to jig-mcp

Thanks for your interest in contributing to jig-mcp!


## Reporting Bugs

Open an issue on GitHub with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Your OS and Go version (`go version`)
- Transport mode (stdio or SSE) and any relevant environment variables

## Suggesting Features

Open a feature request issue. Describe the problem you're trying to solve, not just the feature you want. This helps us find the best solution, which may be different from the initial proposal.

## Submitting Changes

### Branch Naming

Use descriptive branch names with a prefix:

- `fix/` for bug fixes (e.g., `fix/sse-timeout-handling`)
- `feat/` for new features (e.g., `feat/prometheus-metrics`)
- `docs/` for documentation changes (e.g., `docs/tool-developer-guide`)
- `refactor/` for code restructuring (e.g., `refactor/audit-log-rotation`)

### Pull Request Process

1. Fork the repo and create a branch from `main`.
2. Make your changes.
3. Run the full validation suite:
   ```bash
   make all    # runs vet, lint, test-race, build
   ```
4. Add or update tests for any changed behavior.
5. Update documentation if your change affects user-facing behavior (README, CHANGELOG, etc.).
6. Open a pull request against `main`.

Keep PRs focused — one fix or feature per PR. If you find an unrelated issue while working, open a separate PR for it.

### Commit Messages

Write clear, descriptive commit messages. Follow this format:

```
<type>: <short summary>

<optional body explaining the why, not the what>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `test`: Adding or updating tests
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `ci`: Changes to CI/CD configuration
- `chore`: Maintenance tasks (dependency updates, etc.)

Examples:
```
feat: add Prometheus metrics endpoint for tool execution

fix: prevent SSE session leak on client disconnect

docs: add sandbox configuration examples to tool developer guide
```

### What Makes a Good PR

- Tests pass (`make test-race`)
- Linter is happy (`make lint`)
- New functionality has tests
- Breaking changes are documented
- The PR description explains _why_, not just _what_

## Development Setup

### Prerequisites

- Go 1.25+ (see `go.mod` for exact version)
- `golangci-lint` (for linting)

### Getting Started

```bash
git clone https://github.com/snipspin/jig-mcp.git
cd jig-mcp
make build
make test
```

### Running the Full Suite

```bash
make all        # vet + lint + test-race + build
make cover      # generate coverage report
```

### Project Structure

```
cmd/jig-mcp/    Entry point and CLI flags
common/         Shared types (Tool interface, ToolDef)
internal/
  audit/        JSONL audit logging with rotation
  auth/         Token authentication and caller identity
  config/       YAML manifest loading and validation
  dashboard/    Optional HTTP monitoring dashboard
  logging/      Structured slog setup
  rlimit/       Platform-specific resource limits
  server/       JSON-RPC request processing
  sse/          HTTP/SSE transport
  tools/        Tool implementations (External, HTTP, Terminal)
tools/          Example tool manifests
scripts/        Example self-describing scripts
docs/           Developer documentation
testdata/       Test helper binaries
```

### Testing

```bash
make test           # standard test run
make test-race      # with race detector (required for PRs)
make cover          # generate coverage report
```

Tests use `testdata/` for helper binaries that are compiled on demand. Audit log tests use `JIG_LOG_DIR` overrides for isolation, so they can safely run in parallel.

## Code Style

- Run `gofmt` before committing (enforced by CI).
- Follow standard [Go conventions](https://go.dev/doc/effective_go).
- Add godoc comments to all exported types and functions.
- Add tests for new functionality.
- Keep dependencies minimal — jig-mcp has one external dependency (`gopkg.in/yaml.v3`) and we'd like to keep it that way. If you need to add a dependency, explain why in the PR.
- Use `log/slog` for structured logging, not `fmt.Print` or `log.Print`.
- Errors should be wrapped with context using `fmt.Errorf("doing thing: %w", err)`.

## Security

If you discover a security vulnerability, **do not open a public issue**. Please open a private GitHub issue instead.

When writing code, be mindful of:
- Shell injection — never interpolate user input into shell commands
- Path traversal — validate file paths from user input
- SSRF — respect `allowedURLPrefixes` in HTTP tools

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
