# Contributing to cockroachdb-mcp-server

Thanks for your interest! This guide covers what you need to send a PR.

## Getting started

1. Fork the repo, clone your fork, and create a feature branch.
2. Set up the dev environment - see the [README](README.md) for prerequisites
   and the build command.
3. Install [golangci-lint](https://golangci-lint.run) for local lint runs.

## Making changes

- **Format and lint** before pushing:
  ```bash
  gofmt -w .
  go vet ./...
  golangci-lint run ./...
  ```
- **Add tests** for new behavior. We use Go's standard `testing` package with
  `t.Run` subtests and `github.com/stretchr/testify/require` for assertions.
- **Use `github.com/cockroachdb/errors`** for error construction and wrapping;
  do not use `fmt.Errorf` for errors.
- **Default to no comments.** Only add one when the *why* is non-obvious - a
  hidden constraint, a subtle invariant, a workaround for a specific bug.
  Well-named identifiers should explain *what*.
- **Run the full test suite**:
  ```bash
  go test ./...
  ```

## Commit messages and PRs

- One commit per PR. Squash-amend rather than stacking follow-up commits.
- Commit subject: `<area>: <imperative>`, lowercase, under 70 chars
  (e.g. `security: block password-based auth by default`).
- Commit body: one short paragraph explaining the *why* and the *what*.
- PR description follows `## Summary` (one paragraph) plus `## Changes`
  (bulleted file-by-file list). Look at recent merged PRs for examples.
- Reference [SECURITY.md](SECURITY.md) for any change touching auth, DSN
  handling, or SQL validation.

## Reporting bugs

Use [GitHub Issues](https://github.com/cockroachdb/cockroachdb-mcp-server/issues)
for non-security bugs. For security issues, see [SECURITY.md](SECURITY.md).
