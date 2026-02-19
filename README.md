# Wayback Machine HTTP Proxy

A simple HTTP proxy that routes requests through the [Wayback Machine](https://web.archive.org/), with each port corresponding to a specific year (1995-2005).

It's intented to be used for retro-computing demos etc...

This is inspired by `theoldnet.com`'s [HTTP Proxy](https://theoldnet.com/docs/httpproxy/index.html) service.

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

## Testing

Run the unit tests:

```bash
go test -v
```

Run tests with coverage:

```bash
go test -cover
```

Generate detailed coverage report:

```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

The test suite includes:
- **HTML cleaning tests**: Validates removal of Wayback Machine artifacts, script removal, and URL rewriting
- **Proxy handler tests**: Tests HTTP proxy functionality, error handling, and request routing
- **Edge case tests**: Covers empty inputs, malformed HTML, and various quote styles
- **Benchmark tests**: Performance testing for the HTML cleaning function

Current test coverage: ~54% (93.6% coverage of the core [`cleanHTML()`](main.go:24) function)

## Quality
This is all prompted, and I haven't really read much of the the code. Don't use it for anything important.