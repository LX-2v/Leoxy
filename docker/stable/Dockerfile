# syntax=docker/dockerfile:1.7
# Enable modern Dockerfile features such as cache mounts.
# Set the Go toolchain and runtime base image versions.
ARG GO_VERSION=1.27.4

# Build on the requested target platform using a platform-native Go builder.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS builder

# Accept the target platform and application version from BuildKit.
ARG TARGETOS
ARG TARGETARCH
ARG LEOXY_VERSION=1.15.2
# Disable CGO and workspace mode for a portable static build.
ENV CGO_ENABLED=0 GOWORK=off

# Keep application source in the build workspace.
WORKDIR /src/app

# Copy module manifests separately so dependency downloads can be cached.
COPY app/go.mod app/go.sum ./

# Download dependencies using a persistent BuildKit cache.
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the application source after dependency resolution.
COPY app/ ./

# Compile the CLI for the requested platform and embed its release version.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=v${LEOXY_VERSION}" -o /out/leoxy ./cli


# Start a small Debian runtime image without BusyBox.
FROM debian:bookworm-slim

# Add release and source metadata to the resulting image.
ARG LEOXY_VERSION=1.15.2
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown

# Provide standard OCI metadata for image registries and scanners.
LABEL org.opencontainers.image.title="leoxy" \
      org.opencontainers.image.version="${LEOXY_VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/elrefai99/Leoxy" \
      org.opencontainers.image.licenses="Apache-2.0"

# Listen on all container interfaces and use HTTP port 80 by default.
ENV LEOXY_HOST=0.0.0.0 PORT=80

# Install trusted certificates and the health-check client, create an unprivileged account, and prepare writable directories.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 leoxy \
    && useradd --system --uid 10001 --gid leoxy --no-create-home leoxy \
    && mkdir -p /app/leoxy /app/log /app/docs/html \
    && chown -R leoxy:leoxy /app

# Set the runtime working directory.
WORKDIR /app

# Copy the compiled executable and bundled HTML documentation.
COPY --from=builder /out/leoxy /usr/local/bin/leoxy
COPY --chown=leoxy:leoxy docs/html /app/docs/html

# Run the application without root privileges.
USER 10001:10001

# Document the HTTP port used by the service.
EXPOSE 80

# Ask the process to shut down gracefully when the container stops.
STOPSIGNAL SIGTERM

# Check the local liveness endpoint periodically.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null "http://127.0.0.1:${PORT:-80}/health/live" || exit 1

# Launch Leoxy when the container starts.
ENTRYPOINT ["/usr/local/bin/leoxy"]
