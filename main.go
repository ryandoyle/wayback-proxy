package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// version is set at build time via -ldflags
var version = "dev"

const (
	waybackAPIBase = "https://web.archive.org/web"
	startPort      = 1995
	endPort        = 2005
)

// cleanHTML removes Wayback Machine artifacts from HTML content and rewrites URLs
func cleanHTML(body []byte, year int, baseURL string) []byte {
	// Remove Wayback Machine toolbar
	toolbarRe := regexp.MustCompile(`(?s)<!--\s*BEGIN WAYBACK TOOLBAR INSERT\s*-->.*?<!--\s*END WAYBACK TOOLBAR INSERT\s*-->`)
	body = toolbarRe.ReplaceAll(body, []byte(""))

	// Remove all <script> tags from the <head> section
	// This regex matches <head>...</head> and then removes all <script>...</script> within it
	headRe := regexp.MustCompile(`(?si)(<head[^>]*>)(.*?)(</head>)`)
	body = headRe.ReplaceAllFunc(body, func(match []byte) []byte {
		// Extract the parts
		parts := headRe.FindSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		openTag := parts[1]
		headContent := parts[2]
		closeTag := parts[3]

		// Remove all script tags from head content
		scriptRe := regexp.MustCompile(`(?si)<script[^>]*>.*?</script>`)
		cleanedContent := scriptRe.ReplaceAll(headContent, []byte(""))

		// Reconstruct the head section
		return bytes.Join([][]byte{openTag, cleanedContent, closeTag}, []byte(""))
	})

	// For href attributes, simplify Wayback URLs
	// If href="/web/{digits}/...", drop the "/web/{digits}/" and keep just the "..."
	// Otherwise, leave it as is
	hrefRe := regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
	body = hrefRe.ReplaceAllFunc(body, func(match []byte) []byte {
		parts := hrefRe.FindSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		hrefURL := string(parts[1])

		// Check if this matches the pattern /web/{digits}/...
		waybackPathRe := regexp.MustCompile(`^/web/\d+/(.+)$`)
		if matches := waybackPathRe.FindStringSubmatch(hrefURL); len(matches) == 2 {
			// Extract the URL after /web/{digits}/
			cleanedURL := matches[1]
			log.Printf("[Year %d] Rewriting href: %s -> %s", year, hrefURL, cleanedURL)
			return []byte(fmt.Sprintf(`href="%s"`, cleanedURL))
		}

		// Otherwise, leave it as is
		return match
	})

	// For src attributes, simplify Wayback URLs
	// If src="/web/{digits}{2 a-z chars}_/...", prepend "http://web.archive.org"
	// The chars indicate assets. eg "im_" is for images, "js_", javascript. Serve
	// these directly from wayback machine
	srcRe := regexp.MustCompile(`(?i)src=["']([^"']+)["']`)
	body = srcRe.ReplaceAllFunc(body, func(match []byte) []byte {
		parts := srcRe.FindSubmatch(match)
		if len(parts) < 2 {
			return match
		}

		srcURL := string(parts[1])

		// Check if this matches the pattern /web/{digits}{2 a-z chars}_/...
		waybackWithFlagsRe := regexp.MustCompile(`^(/web/\d+[a-z]{2}_/.+)$`)
		if matches := waybackWithFlagsRe.FindStringSubmatch(srcURL); len(matches) == 2 {
			// Prepend http://web.archive.org
			fullURL := "http://web.archive.org" + matches[1]
			log.Printf("[Year %d] Rewriting src with flags: %s -> %s", year, srcURL, fullURL)
			return []byte(fmt.Sprintf(`src="%s"`, fullURL))
		}

		// Otherwise, leave it as is
		return match
	})

	return body
}

// ProxyHandler handles HTTP proxy requests for a specific year
type ProxyHandler struct {
	year int
}

// ServeHTTP implements the http.Handler interface
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// For HTTP PROXY mode, we need to handle CONNECT for HTTPS and direct requests for HTTP
	if r.Method == http.MethodConnect {
		// HTTPS tunneling - not supported for Wayback Machine
		http.Error(w, "HTTPS proxying not supported", http.StatusMethodNotAllowed)
		return
	}

	// Get the target URL from the request
	targetURL := r.URL.String()

	// If the URL doesn't have a scheme, it might be in the Host header (proxy mode)
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if r.Host != "" {
			targetURL = "http://" + r.Host + r.URL.String()
		} else {
			http.Error(w, "Invalid request URL", http.StatusBadRequest)
			return
		}
	}

	// Construct the Wayback Machine URL
	waybackURL := fmt.Sprintf("%s/%d/%s", waybackAPIBase, p.year, targetURL)

	log.Printf("[Port %d - Year %d] Proxying: %s -> %s",
		startPort+(p.year-startPort), p.year, targetURL, waybackURL)

	// Create a new request to the Wayback Machine
	proxyReq, err := http.NewRequest(r.Method, waybackURL, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request (excluding hop-by-hop headers)
	for name, values := range r.Header {
		// Skip hop-by-hop headers
		if name == "Connection" || name == "Keep-Alive" || name == "Proxy-Authenticate" ||
			name == "Proxy-Authorization" || name == "Te" || name == "Trailers" ||
			name == "Transfer-Encoding" || name == "Upgrade" {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute the request
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow redirects but limit to 10
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching from Wayback Machine: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Handle 4XX and 5XX errors from Wayback Machine
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		// Client errors (404 Not Found, etc.)
		body, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("Wayback Machine client error (HTTP %d): The requested page may not be archived for year %d. URL: %s",
			resp.StatusCode, p.year, targetURL)
		if len(body) > 0 && len(body) < 500 {
			errorMsg += fmt.Sprintf("\nDetails: %s", string(body))
		}
		log.Printf("[Port %d - Year %d] Client error %d for %s",
			startPort+(p.year-startPort), p.year, resp.StatusCode, targetURL)
		http.Error(w, errorMsg, resp.StatusCode)
		return
	}

	if resp.StatusCode >= 500 {
		// Server errors
		body, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("Wayback Machine server error (HTTP %d): The archive service may be temporarily unavailable.",
			resp.StatusCode)
		if len(body) > 0 && len(body) < 500 {
			errorMsg += fmt.Sprintf("\nDetails: %s", string(body))
		}
		log.Printf("[Port %d - Year %d] Server error %d for %s",
			startPort+(p.year-startPort), p.year, resp.StatusCode, targetURL)
		http.Error(w, errorMsg, http.StatusBadGateway)
		return
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading response body: %v", err), http.StatusBadGateway)
		return
	}

	// Check if this is HTML content and clean it
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		body = cleanHTML(body, p.year, targetURL)
	}

	// Copy response headers
	for name, values := range resp.Header {
		// Skip hop-by-hop headers
		if name == "Connection" || name == "Keep-Alive" || name == "Proxy-Authenticate" ||
			name == "Proxy-Authorization" || name == "Te" || name == "Trailers" ||
			name == "Transfer-Encoding" || name == "Upgrade" {
			continue
		}
		// Update Content-Length if we modified the body
		if name == "Content-Length" && strings.Contains(strings.ToLower(contentType), "text/html") {
			continue // We'll set this ourselves
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set the correct Content-Length for the modified body
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Write the response body
	_, err = w.Write(body)
	if err != nil {
		log.Printf("Error writing response body: %v", err)
	}
}

// startProxyServer starts a proxy server on the given port for the given year
func startProxyServer(port int) error {
	year := port // The port number IS the year
	handler := &ProxyHandler{year: year}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	log.Printf("Starting Wayback Machine proxy on port %d (year %d)", port, year)
	return server.ListenAndServe()
}

func main() {
	log.Printf("Wayback Machine HTTP Proxy version %s", version)

	var wg sync.WaitGroup

	// Start a proxy server for each year/port from 1995 to 2005
	for port := startPort; port <= endPort; port++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			if err := startProxyServer(p); err != nil {
				log.Printf("Error starting server on port %d: %v", p, err)
			}
		}(port)
	}

	log.Printf("Wayback Machine HTTP Proxy started")
	log.Printf("Listening on ports %d-%d (years %d-%d)", startPort, endPort, startPort, endPort)
	log.Printf("Configure your browser to use HTTP proxy localhost:<port> where port is the year you want")

	// Wait for all servers
	wg.Wait()
}
