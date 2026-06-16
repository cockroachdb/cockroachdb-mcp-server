# Pinned to the multi-arch index digest so the base image cannot change
# underneath us. Dependabot refreshes the digest.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639

COPY cockroachdb-mcp-server /usr/local/bin/cockroachdb-mcp-server

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/cockroachdb-mcp-server"]
