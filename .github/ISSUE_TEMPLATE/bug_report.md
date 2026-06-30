---
name: Bug report
about: Report a defect in cockroachdb-mcp-server
title: ''
labels: bug
assignees: ''

---

**Environment**

- `cockroachdb-mcp-server --version`:
- CockroachDB version (`SELECT version()`):
- Transport (`CRDB_MCP_TRANSPORT`): stdio or http
- Deployment (binary, Docker image, `go install`):
- Auth mode (cert or password - do **not** paste any credentials).
  Password mode requires `CRDB_MCP_ALLOW_PASSWORD_AUTH=true`:
- MCP client (Claude Desktop, Cursor, custom, etc.):

**Describe the bug**
A clear description of what went wrong.

**To reproduce**
Steps to reproduce, ideally with a minimal MCP tool invocation or SQL.

**Expected behavior**
What you expected to happen.

**Actual behavior**
What actually happened. Include error messages or log output when relevant
(set `CRDB_MCP_LOG_LEVEL=debug` for more detail).

**Additional context**
Anything else worth knowing (network topology, reverse proxy, k8s manifest snippets, etc.).
