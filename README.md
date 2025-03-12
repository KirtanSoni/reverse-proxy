# Dynamic Reverse Proxy Server

A flexible, runtime-configurable reverse proxy server built in Go with automatic TLS certificate management.

## Features

- **Dynamic Proxy Management**: Add or remove proxy routes at runtime through a simple CLI
- **Automatic TLS**: Built-in Let's Encrypt integration for automatic certificate issuance and renewal
- **Secure by Default**: Implements security headers and modern TLS configurations
- **Graceful Shutdown**: Properly handles termination signals for zero-downtime deployments
- **Configurable Timeouts**: Fine-tune performance with customizable server timeouts

## Installation

### Prerequisites

- Go 1.16 or higher
- Domain name with DNS configured to point to your server

### Build from Source

```bash
git clone https://github.com/kirtansoni/reverse-proxy-go.git
cd reverse-proxy-go
go build -o proxy-server
```

## Usage

### Command Line Options

```
Usage:
  ./proxy-server [options]

Options:
  -http string         HTTP address (default ":80")
  -https string        HTTPS address (default ":443")
  -domain string       Domain name (required)
  -certdir string      Directory to store Let's Encrypt certificates (default "./certs")
  -read-timeout        Read timeout (default 5s)
  -write-timeout       Write timeout (default 10s)
  -idle-timeout        Idle timeout (default 120s)
  -max-header-bytes    Max header bytes (default 1MB)
  -shutdown-timeout    Shutdown timeout (default 30s)
```

### Basic Example

```bash
./proxy-server -domain example.com
```

### Advanced Example

```bash
./proxy-server -domain example.com -certdir /etc/certs -http :8080 -https :8443 -read-timeout 10s -write-timeout 20s
```

## Runtime Management

Once the server is running, you can manage proxy routes through the interactive CLI:

- **Add a new route**: `add <name> <path> <target_url>`
- **Remove a route**: `remove <path>`
- **List all routes**: `list`
- **Exit CLI**: `exit`

Example CLI session:
```
> add API /api https://api-backend.example.com
Added service API at path /api
> add Static /static https://storage.example.com/static-assets
Added service Static at path /static
> list
===== Services of RunTime Mux ========
{"name":"API","path":"/api","url":"https://api-backend.example.com"}
{"name":"Static","path":"/static","url":"https://storage.example.com/static-assets"}
======================================
> remove /api
Removed service at path /api
> exit
```

## Architecture

The server consists of three main components:

1. **Main Server**: Handles HTTP/HTTPS requests and manages TLS certificates
2. **Proxy Package**: Implements the dynamic reverse proxy with runtime configuration
3. **SSL Package**: Manages TLS certificates and security settings

All requests to `/projects/*` are handled by the dynamic proxy, while the root path (`/`) is handled by a static handler.

## Security Features

- Automatic TLS certificate management via Let's Encrypt
- Modern TLS configuration with strong cipher suites
- Essential security headers:
  - X-Frame-Options
  - X-Content-Type-Options
  - X-XSS-Protection
  - Referrer-Policy
  - Content-Security-Policy
  - Strict-Transport-Security

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.