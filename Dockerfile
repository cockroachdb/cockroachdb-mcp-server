# Pinned to the multi-arch index digest so the base image cannot change
# underneath us. Dependabot refreshes the digest.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:d093aa3e30dbadd3efe1310db061a14da60299baff8450a17fe0ccc514a16639

# dockers_v2 lays per-platform binaries at <os>/<arch>/<name>.
ARG TARGETOS
ARG TARGETARCH
COPY ${TARGETOS}/${TARGETARCH}/cockroachdb-mcp-server /usr/local/bin/cockroachdb-mcp-server

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/cockroachdb-mcp-server"]
