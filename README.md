# Wayback Machine HTTP Proxy

A simple HTTP proxy that routes requests through the Wayback Machine, with each port corresponding to a specific year (1995-2005).

## Quick Start

```bash
go run main.go
```

The proxy starts on ports 1995-2005, where each port number represents that year's archive.

## Usage

Configure your browser or application to use HTTP proxy `localhost` with a port between 1995-2005.

```bash
# Example with curl
curl --proxy http://localhost:2000 http://example.com
```

This fetches example.com as it appeared in the year 2000.

### Important: Proxy Exception

Add `web.archive.org` to your proxy exception list to prevent infinite loops. This proxy will re-write `src` 
links to the direct `web.archive.org/...` links as the proxy needs to access the Wayback Machine directly 
without going through itself.

## Quality
This is all prompted, and I haven't really read the code. Don't use it for anything important.