# syntax=docker/dockerfile:1.7
# Enable modern Dockerfile features.
# Choose the Go toolchain version.
ARG GO_VERSION=1.27.4
# Use Debian Bookworm as the CI build environment.
FROM golang:${GO_VERSION}-bookworm

# Disable interactive package prompts and embedded VCS metadata.
ENV DEBIAN_FRONTEND=noninteractive GOFLAGS=-buildvcs=false

# Install build tools and remove apt package lists to reduce image size.
RUN apt-get update \
    && apt-get install -y --no-install-recommends git make gcc libc6-dev ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create a dedicated unprivileged build account.
RUN groupadd -g 1000 leoxybuild \
    && useradd -m -u 1000 -g 1000 -s /bin/bash leoxybuild

# Set the project working directory.
WORKDIR /workspace

# Install the repository CI entrypoint script.
COPY --chmod=755 docker/build/entrypoint.sh /usr/local/bin/entrypoint.sh

# Run build commands as the unprivileged account.
USER leoxybuild

# Execute the CI entrypoint when the helper starts.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
