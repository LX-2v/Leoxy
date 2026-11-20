# Select the Alpine-based Go toolchain version.
ARG GO_VERSION=1.27.4-alpine

# Compile Leoxy in a dedicated builder stage.
FROM golang:${GO_VERSION} AS builder

# Set the module workspace.
WORKDIR /app/app

# Copy module manifests separately to cache dependency downloads.
COPY app/go.mod app/go.sum ./
# Download Go dependencies.
RUN go mod download

# Copy repository source and documentation into the builder.
WORKDIR /app
COPY . .

# Build a static Linux executable without the Go workspace file.
WORKDIR /app/app
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/leoxy ./cli

# Use the Alpine Go image as the runtime base.
FROM golang:${GO_VERSION}

# Listen on all container interfaces.
ENV LEOXY_HOST=0.0.0.0 PORT=80

# Set the runtime working directory.
WORKDIR /app

# Create the runtime account.
RUN addgroup -S leoxy && adduser -S -G leoxy leoxy

# Copy the executable and bundled HTML documentation from the builder.
COPY --from=builder /out/leoxy /usr/local/bin/leoxy
COPY --from=builder --chown=leoxy:leoxy /app/docs/html /app/docs/html

# Run the service without root privileges.
USER leoxy

# Document the HTTP service port.
EXPOSE 80

# Check the service's local liveness endpoint.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:80/health/live || exit 1

# Launch Leoxy when the container starts.
ENTRYPOINT ["/usr/local/bin/leoxy"]
