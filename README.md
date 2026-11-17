<div align="center">

  <img src="./docs/_static/leoxy.png" width="200" alt="Logo" />

<p>

  [![CI](https://github.com/LX-2v/Leoxy/actions/workflows/ci.yml/badge.svg)](https://github.com/LX-2v/Leoxy/actions/workflows/ci.yml)
  [![Docker](https://github.com/LX-2v/Leoxy/actions/workflows/docker.yml/badge.svg)](https://github.com/LX-2v/Leoxy/actions/workflows/docker.yml)
  [![Go version](https://img.shields.io/badge/Go-1.27.4-00ADD8?logo=go)](https://go.dev/dl/)
  [![Docker pulls](https://img.shields.io/docker/pulls/mohamed0699/leoxy)](https://hub.docker.com/r/mohamed0699/leoxy)
  <br/>
  A lightweight reverse proxy built in Go.
  <br/>
</p>
</div>

---
<p> LX-2 routes HTTP requests to configurable upstream servers based on path prefixes. It supports HTTP/1.1, HTTP/2 upstreams, WebSocket upgrades, GraphQL over HTTP, and raw TCP/UDP traffic through dedicated listeners.</p>

## Deployment architecture

LX-2 routes client requests to configured application servers.

```text
Client -> Leoxy -> Application servers
```

## Quick Start

### Using Docker

```bash
docker pull mohamed0699/leoxy:latest
docker run --rm -p 80:80 -v "${PWD}/leoxy:/app/leoxy" mohamed0699/leoxy:latest
```

### Using Docker Compose

```yaml
services:
  leoxy:
    image: mohamed0699/leoxy:latest
    ports:
      - "80:80"
    volumes:
      - ./leoxy:/app/leoxy
    restart: unless-stopped
```

```bash
docker compose up -d
```


### Docker Networking

When Leoxy runs inside a Docker container, `localhost` refers to the Leoxy container itself, not the Docker host or other containers.

Choose the upstream address based on where your application is running.

#### Applications running on the host machine

Use `host.docker.internal` to reach services running directly on your host:

```yaml
upstream:
  - name: "Backend"
    protocol: "http"
    path: "/api"
    server_url: "http://host.docker.internal:9000"
```

On Linux Docker Engine, add the host gateway mapping:

```yaml
services:
  leoxy:
    image: mohamed0699/leoxy:latest
    ports:
      - "80:80"
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

The backend must listen on `0.0.0.0` rather than `127.0.0.1` to accept connections from the container.

#### Applications running in Docker Compose

Use the application service name as the hostname. Docker Compose provides DNS-based service discovery between containers on the same network.

```yaml
services:
  leoxy:
    image: mohamed0699/leoxy:latest
    ports:
      - "80:80"
    volumes:
      - ./leoxy:/app/leoxy
    depends_on:
      - backend
    restart: unless-stopped

  backend:
    build: ./backend
    expose:
      - "9000"
    restart: unless-stopped
```

Configure Leoxy to use the backend service name:

```yaml
upstream:
  - name: "Backend"
    protocol: "http"
    path: "/api"
    server_url: "http://backend:9000"
```

**Important:**
- `localhost` refers to the current container.
- `host.docker.internal` refers to the Docker host.
- `backend` refers to another service on the same Compose network.
- Both containers must share a Docker network to communicate.
- Container-to-container communication uses the container port, not the published host port.

### Building from source

```bash
go build -o leoxy ./app/cli
./leoxy
```

## Configuration

Leoxy reads configuration from `leoxy/leoxy_config.yaml`. On first start, a default config is created. See the [configuration guide](docs/configuration.md) for the full field reference and setup instructions.

### Basic example

```yaml
server:
  port: "80"
  security:
    global_rate_limit: 10000
    global_rate_burst: 1000

upstream:
  - name: "Backend"
    path: /api
    servers:
      - "http://localhost:7000"
      - "http://localhost:7001"
    security:
      rate_limit: 1000
      rate_limit_burst: 100
      route_rate_limit: 1000
      route_rate_burst: 100

  - name: "Frontend"
    path: /
    server_url: "http://localhost:3000"
    security:
      rate_limit: 1000
      rate_limit_burst: 500
```

`global_rate_limit` and `global_rate_burst` define one shared HTTP limit. An upstream's `security.rate_limit` and `rate_limit_burst` define its independent per-IP limit; `route_rate_limit` and `route_rate_burst` define an additional shared per-route limit. A root frontend with no per-IP fields is protected by the global limit only. For compatibility, a non-root upstream with a missing or zero per-IP rate inherits server-level `security.rate_limit` and `rate_limit_burst`; an upstream's positive per-IP rate takes precedence.

The global bucket is charged before route dispatch, so a request rejected by a later route policy still consumes one global token. A rejection inside the route policy does not consume tokens from its other per-IP or per-route buckets.

### TCP/UDP example

```yaml
upstream:
  - name: "GameTCP"
    protocol: tcp
    listen: ":7001"
    server_url: "127.0.0.1:7000"

  - name: "MetricsUDP"
    protocol: udp
    listen: ":7002"
    server_url: "127.0.0.1:7003"
```

## Features

- Reverse proxy with path-based routing
- Round-robin load balancing with failover
- Health checks (`/health/live`, `/health/ready`)
- Ping endpoint (`/ping`)
- Request/response metrics (`/metrics`)
- Rate limiting (per-IP, per-route, global)
- API key and JWT authentication
- mTLS support
- Request body size limits
- CIDR allow/deny lists
- WebSocket and GraphQL support
- TCP and UDP proxying

## CLI

```bash
leoxy             # Run in foreground
leoxy start       # Start in background
leoxy stop        # Stop background process
leoxy version     # Print version
```

## Security

- Rate limits are enforced per-minute with burst capacity
- Frontend and API per-IP limits are independent; the global HTTP limit remains shared
- Redis can be used for the distributed global token bucket; `redis_fail_open: true` falls back to the local global bucket when Redis is unavailable
- API keys via `X-API-Key` header or Bearer token
- JWT authentication with HS256 (requires `exp`)
- mTLS support behind TLS terminator
- CIDR-based access control
- Request body limits

Set secrets via environment variables: `API_KEYS`, `JWT_SECRET`, `JWT_ISSUER`, `JWT_AUDIENCE`, `REDIS_PASSWORD`. Override port with `PORT`. `RATE_LIMIT` remains a compatibility override for the server-level legacy per-IP policy inherited by non-root upstreams; configure current per-route policies under each upstream's `security` block.

## Documentation

- [Leoxy overview and usage](docs/leoxy.md)
- [Configuration reference](docs/configuration.md)
- [CLI commands](docs/cli.md)
- [Docker deployment](docs/docker.md)
- [Benchmark tool](docs/benchmark.md)
- [Resource watcher](docs/watcher/watcher.md)


# TypeScript

[![CI](https://github.com/microsoft/TypeScript/actions/workflows/ci.yml/badge.svg)](https://github.com/microsoft/TypeScript/actions/workflows/ci.yml)
[![npm version](https://badge.fury.io/js/typescript.svg)](https://www.npmjs.com/package/typescript)
[![Downloads](https://img.shields.io/npm/dm/typescript.svg)](https://www.npmjs.com/package/typescript)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/microsoft/TypeScript/badge)](https://securityscorecards.dev/viewer/?uri=github.com/microsoft/TypeScript)

[TypeScript](https://www.typescriptlang.org/) is a language for application-scale JavaScript. TypeScript adds optional types to JavaScript that support tools for large-scale JavaScript applications for any browser, for any host, on any OS. TypeScript compiles to readable, standards-based JavaScript. Try it out at the [playground](https://www.typescriptlang.org/play/), and stay up to date via [our blog](https://devblogs.microsoft.com/typescript/) and [Bluesky](https://bsky.app/profile/typescriptlang.org).

Find others who are using TypeScript at [our community page](https://www.typescriptlang.org/community/).

## Installing

For the latest stable version:

```bash
npm install -D typescript
```

For our nightly builds:

```bash
npm install -D typescript@next
```

## Contribute

There are many ways to [contribute](https://github.com/microsoft/TypeScript/blob/main/CONTRIBUTING.md) to TypeScript.

* [Submit bugs](https://github.com/microsoft/TypeScript/issues) and help us verify fixes as they are checked in.
* Review the [source code changes](https://github.com/microsoft/TypeScript/pulls).
* Engage with other TypeScript users and developers on [Stack Overflow](https://stackoverflow.com/questions/tagged/typescript).
* Help each other in the [TypeScript Community Discord](https://discord.gg/typescript).
* Join the [#typescript](https://bsky.app/search?q=%23typescript) discussion on Bluesky.
* [Contribute bug fixes](https://github.com/microsoft/TypeScript/blob/main/CONTRIBUTING.md).

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/). For more information see
the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or contact [opencode@microsoft.com](mailto:opencode@microsoft.com)
with any additional questions or comments.

## Documentation

* [TypeScript in 5 minutes](https://www.typescriptlang.org/docs/handbook/typescript-in-5-minutes.html)
* [Programming handbook](https://www.typescriptlang.org/docs/handbook/intro.html)
* [Homepage](https://www.typescriptlang.org/)

## Roadmap

For details on our planned features and future direction, please refer to our [roadmap](https://github.com/microsoft/TypeScript/wiki/Roadmap).
