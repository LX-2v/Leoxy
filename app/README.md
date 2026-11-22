# LX-2 App

Core application code for the LX-2 reverse proxy.

## Building

```bash
go build -o leoxy ./app/cli
```

## Running

```bash
./leoxy             # Foreground
./leoxy start       # Run Leoxy in Background
./leoxy restart     # Restart Leoxy in Background
./leoxy stop        # Stop
./leoxy version     # Version
```

## Module

```go
module github.com/elrefai99/Leoxy/app
```

## Version

Current version: `v1.15.1` (defined in `app/cli/main.go`)
