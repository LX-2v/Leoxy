#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
KEY="e2e-key"
PIDS=()

cleanup() {
     for pid in "${PIDS[@]}"; do
          kill "$pid" 2>/dev/null || true
     done
     if [ -f "$WORK/leoxy/keys/leoxy.pid" ]; then
          kill "$(cat "$WORK/leoxy/keys/leoxy.pid")" 2>/dev/null || true
     fi
     rm -rf "$WORK"
}
trap cleanup EXIT

cd "$ROOT" || exit 1

go build -o "$WORK/leoxy-bin" ./app/cli || exit 1
mkdir -p "$WORK/leoxy"
cp script/test/config.e2e.yaml "$WORK/leoxy/config.yaml"

python3 script/test/upstream.sh &
PIDS+=("$!")

( cd "$WORK" && exec env API_KEYS="$KEY" ./leoxy-bin run ) &
PIDS+=("$!")

ready=0
for _ in $(seq 40); do
     if curl -s -o /dev/null -H "X-API-Key: $KEY" http://127.0.0.1:80/ping; then
          ready=1
          break
     fi
     sleep 0.5
done

if [ "$ready" -ne 1 ]; then
     echo "Leoxy did not become ready"
     exit 1
fi

API_KEY="$KEY" \
EP1=/api/test EP2=/ EP3=/health/ready \
LB_HEADER=X-Served-By \
BODY_EP=/body BODY_LIMIT_MB=1 \
RL_EPS="/limited-a /limited-b /limited-c" \
DENIED_METHOD=TRACE \
TCP_ADDR=127.0.0.1:7001 TCP_ECHO=1 \
UDP_ADDR=127.0.0.1:7002 \
N=100 C=20 \
bash script/test/run-all.sh

kill "${PIDS[1]}" 2>/dev/null || true
wait "${PIDS[1]}" 2>/dev/null || true
PIDS=("${PIDS[0]}")

( cd "$WORK" && env API_KEYS="$KEY" ./leoxy-bin --config "$WORK/leoxy/config.yaml" --data-dir "$WORK" start ) || exit 1

ready=0
for _ in $(seq 40); do
     if curl -s -o /dev/null -H "X-API-Key: $KEY" http://127.0.0.1:80/ping; then
          ready=1
          break
     fi
     sleep 0.5
done

if [ "$ready" -ne 1 ]; then
     echo "Leoxy did not become ready for stop test"
     exit 1
fi

curl -sS -o "$WORK/slow-response" -w '%{http_code}' --max-time 10 -H "X-API-Key: $KEY" http://127.0.0.1:80/slow >"$WORK/slow-status" &
request_pid=$!
sleep 0.5
( cd "$WORK" && ./leoxy-bin --config "$WORK/leoxy/config.yaml" --data-dir "$WORK" stop ) || exit 1
wait "$request_pid" || exit 1

if [ "$(cat "$WORK/slow-status")" != "200" ]; then
     echo "Held request did not complete successfully across stop"
     exit 1
fi
if [ "$(cat "$WORK/slow-response")" != "ok:backend-3000" ]; then
     echo "Held request returned an unexpected response across stop"
     exit 1
fi

echo "PASS  held request completes across stop"
