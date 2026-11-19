#!/usr/bin/env bash
# Exit on errors, unset variables, and failed pipeline commands.
set -Eeuo pipefail

# Use module-local dependency resolution instead of the workspace file.
export GOWORK=off

# Verify, format-check, vet, and race-test each Go module.
for module in app test/benchmark; do
    echo "==> $module"
    pushd "$module" >/dev/null
    go mod verify
    if [ -n "$(gofmt -l .)" ]; then
        gofmt -l .
        echo "Formatting check failed"
        exit 1
    fi
    go vet ./...
    CGO_ENABLED=1 go test -race ./...
    popd >/dev/null
done

# Build the CLI after all module checks pass.
echo "==> Building Leoxy"
mkdir -p dist

(cd app && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../dist/leoxy ./cli)

echo "==> CI completed successfully"
