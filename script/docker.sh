#!/usr/bin/env bash
set -Eeuo pipefail

image=${1:?usage: docker.sh <image>}
port=${SMOKE_PORT:-80}
nameDocker=leoxy-docker-$$
namePodman=leoxy-podman-$$
url=http://127.0.0.1:$port

cleanup() {
	docker rm -f "$nameDocker" >/dev/null 2>&1 || true
	podman rm -f "$namePodman" >/dev/null 2>&1 || true
}
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; docker logs "$nameDocker" >&2 || true; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

docker run -d --name "$nameDocker" --health-interval=2s --cap-add=NET_BIND_SERVICE -p "127.0.0.1:$port:80" \
	-e PORT=80 -e LEOXY_HOST=0.0.0.0 "$image" >/dev/null

status=starting
for _ in {1..60}; do
	status=$(docker inspect -f '{{.State.Health.Status}}' "$nameDocker" 2>/dev/null || true)
	[ "$status" = healthy ] && break
	sleep 1
done
[ "$status" = healthy ] || fail "health status is $status"

[ "$(code "$url/health/live")" = 200 ] || fail "health endpoint did not return 200"
[ "$(curl -fsS "$url/ping")" = PONG ] || fail "ping endpoint did not return PONG"
[ "$(docker exec "$nameDocker" id -u)" != 0 ] || fail "container should not run as root"

echo "Docker work"

fail() { echo "FAIL: $*" >&2; podman logs "$namePodman" >&2 || true; exit 1; }
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }

podman run -d --name "$namePodman" --health-interval=2s --cap-add=NET_BIND_SERVICE -p "127.0.0.1:$port:80" \
	-e PORT=80 -e LEOXY_HOST=0.0.0.0 "$image" >/dev/null

status=starting
for _ in {1..60}; do
	status=$(podman inspect -f '{{.State.Health.Status}}' "$namePodman" 2>/dev/null || true)
	[ "$status" = healthy ] && break
	sleep 1
done
[ "$status" = healthy ] || fail "health status is $status"

[ "$(code "$url/health/live")" = 200 ] || fail "health endpoint did not return 200"
[ "$(curl -fsS "$url/ping")" = PONG ] || fail "ping endpoint did not return PONG"
[ "$(podman exec "$namePodman" id -u)" != 0 ] || fail "container should not run as root"

echo "Podman work"
