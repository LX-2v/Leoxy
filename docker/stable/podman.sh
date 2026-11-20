#!/usr/bin/env bash
# Exit on errors, unset variables, and failed pipeline commands.
set -Eeuo pipefail

# Read the image name and choose a local smoke-test port and container name.
image=${1:?usage: podman.sh <image>}
port=${SMOKE_PORT:-80}
name=leoxy-podman-$$
url=http://127.0.0.1:$port


# Remove the temporary container whether the script succeeds or fails.
cleanup() { podman rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT
# Print a failure, include container logs, and exit with an error.
fail() { echo "FAIL: $*" >&2; podman logs "$name" >&2 || true; exit 1; }
# Return only the HTTP response status code.
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

# Start the image with its HTTP port bound only on localhost.
podman run -d --name "$name" --health-interval=2s --cap-add=NET_BIND_SERVICE -p "127.0.0.1:$port:80" \
	-e PORT=80 -e LEOXY_HOST=0.0.0.0 "$image" >/dev/null

# Wait up to one minute for the container health check to pass.
status=starting
for _ in {1..60}; do
	status=$(podman inspect -f '{{.State.Health.Status}}' "$name" 2>/dev/null || true)
	[ "$status" = healthy ] && break
	sleep 1
done
# Require healthy status and successful liveness and ping endpoints.
[ "$status" = healthy ] || fail "health status is $status"

[ "$(code "$url/health/live")" = 200 ] || fail "health endpoint did not return 200"
[ "$(curl -fsS "$url/ping")" = PONG ] || fail "ping endpoint did not return PONG"
# Verify the application process runs as a non-root user.
[ "$(podman exec "$name" id -u)" != 0 ] || fail "container should not run as root"

# Report successful smoke checks.
echo "ok"
