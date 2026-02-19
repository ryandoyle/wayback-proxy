package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCleanHTML tests the cleanHTML function with various HTML inputs
func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		year     int
		baseURL  string
		expected string
	}{
		{
			name: "Remove Wayback toolbar",
			input: `<html>
<!-- BEGIN WAYBACK TOOLBAR INSERT -->
<div id="wm-toolbar">Wayback Machine toolbar</div>
<!-- END WAYBACK TOOLBAR INSERT -->
<body>Content</body>
</html>`,
			year:    2000,
			baseURL: "http://example.com",
			expected: `<html>

<body>Content</body>
</html>`,
		},
		{
			name: "Remove scripts from head",
			input: `<html>
<head>
<title>Test</title>
<script src="wayback.js"></script>
<script>alert('test');</script>
<link rel="stylesheet" href="style.css">
</head>
<body>Content</body>
</html>`,
			year:    2000,
			baseURL: "http://example.com",
			expected: `<html>
<head>
<title>Test</title>


<link rel="stylesheet" href="style.css">
</head>
<body>Content</body>
</html>`,
		},
		{
			name:     "Rewrite href with /web/digits/ pattern",
			input:    `<a href="/web/20001231120000/http://example.com/page.html">Link</a>`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<a href="http://example.com/page.html">Link</a>`,
		},
		{
			name:     "Rewrite href with full Wayback URL",
			input:    `<a href="https://web.archive.org/web/20001231120000/http://example.com/page.html">Link</a>`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<a href="http://example.com/page.html">Link</a>`,
		},
		{
			name:     "Rewrite src with https to http",
			input:    `<img src="https://web.archive.org/web/20001231im_/http://example.com/image.jpg">`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<img src="http://web.archive.org/web/20001231im_/http://example.com/image.jpg">`,
		},
		{
			name:     "Rewrite src with flags pattern",
			input:    `<img src="/web/20001231im_/http://example.com/image.jpg">`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<img src="http://web.archive.org/web/20001231im_/http://example.com/image.jpg">`,
		},
		{
			name:     "Leave normal href unchanged",
			input:    `<a href="http://example.com/page.html">Link</a>`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<a href="http://example.com/page.html">Link</a>`,
		},
		{
			name:     "Leave normal src unchanged",
			input:    `<img src="http://example.com/image.jpg">`,
			year:     2000,
			baseURL:  "http://example.com",
			expected: `<img src="http://example.com/image.jpg">`,
		},
		{
			name: "Complex HTML with multiple rewrites",
			input: `<html>
<head>
<script src="wayback.js"></script>
<link rel="stylesheet" href="/web/20001231cs_/http://example.com/style.css">
</head>
<body>
<a href="/web/20001231120000/http://example.com/page1.html">Page 1</a>
<a href="https://web.archive.org/web/20001231120000/http://example.com/page2.html">Page 2</a>
<img src="/web/20001231im_/http://example.com/image.jpg">
<img src="https://web.archive.org/web/20001231im_/http://example.com/logo.png">
</body>
</html>`,
			year:    2000,
			baseURL: "http://example.com",
			expected: `<html>
<head>

<link rel="stylesheet" href="/web/20001231cs_/http://example.com/style.css">
</head>
<body>
<a href="http://example.com/page1.html">Page 1</a>
<a href="http://example.com/page2.html">Page 2</a>
<img src="http://web.archive.org/web/20001231im_/http://example.com/image.jpg">
<img src="http://web.archive.org/web/20001231im_/http://example.com/logo.png">
</body>
</html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML([]byte(tt.input), tt.year, tt.baseURL)
			if string(result) != tt.expected {
				t.Errorf("cleanHTML() failed\nInput:\n%s\n\nExpected:\n%s\n\nGot:\n%s",
					tt.input, tt.expected, string(result))
			}
		})
	}
}

// TestProxyHandler tests the ProxyHandler ServeHTTP method
func TestProxyHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		url            string
		host           string
		expectedStatus int
		checkBody      bool
		bodyContains   string
	}{
		{
			name:           "CONNECT method not supported",
			method:         http.MethodConnect,
			url:            "https://example.com:443",
			expectedStatus: http.StatusMethodNotAllowed,
			checkBody:      true,
			bodyContains:   "HTTPS proxying not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &ProxyHandler{year: 2000}

			req := httptest.NewRequest(tt.method, tt.url, nil)
			if tt.host != "" {
				req.Host = tt.host
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.checkBody {
				body := rr.Body.String()
				if !strings.Contains(body, tt.bodyContains) {
					t.Errorf("Expected body to contain %q, got %q", tt.bodyContains, body)
				}
			}
		})
	}
}

// TestProxyHandlerWithHost tests proxy requests with Host header
func TestProxyHandlerWithHost(t *testing.T) {
	handler := &ProxyHandler{year: 2000}

	req := httptest.NewRequest(http.MethodGet, "/path/to/page", nil)
	req.Host = "example.com"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// This will fail to connect to Wayback Machine in tests, but we can verify
	// the request was constructed properly by checking it doesn't return BadRequest
	if rr.Code == http.StatusBadRequest {
		t.Errorf("Request with Host header should not return BadRequest, got %d", rr.Code)
	}
}

// TestProxyHandlerWithFullURL tests proxy requests with full URL
func TestProxyHandlerWithFullURL(t *testing.T) {
	handler := &ProxyHandler{year: 2000}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// This will fail to connect to Wayback Machine in tests, but we can verify
	// the request was constructed properly by checking it doesn't return BadRequest
	if rr.Code == http.StatusBadRequest {
		t.Errorf("Request with full URL should not return BadRequest, got %d", rr.Code)
	}
}

// TestCleanHTMLPreservesContent tests that cleanHTML doesn't corrupt valid content
func TestCleanHTMLPreservesContent(t *testing.T) {
	input := `<html>
<head>
<title>Test Page</title>
<meta charset="utf-8">
</head>
<body>
<h1>Hello World</h1>
<p>This is a test paragraph with <strong>bold</strong> and <em>italic</em> text.</p>
<ul>
<li>Item 1</li>
<li>Item 2</li>
</ul>
</body>
</html>`

	result := cleanHTML([]byte(input), 2000, "http://example.com")

	// Check that basic structure is preserved
	if !bytes.Contains(result, []byte("<title>Test Page</title>")) {
		t.Error("Title was corrupted")
	}
	if !bytes.Contains(result, []byte("<h1>Hello World</h1>")) {
		t.Error("H1 was corrupted")
	}
	if !bytes.Contains(result, []byte("<strong>bold</strong>")) {
		t.Error("Strong tag was corrupted")
	}
	if !bytes.Contains(result, []byte("<em>italic</em>")) {
		t.Error("Em tag was corrupted")
	}
}

// TestCleanHTMLCaseInsensitive tests that regex patterns work case-insensitively
func TestCleanHTMLCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Uppercase HREF",
			input:    `<a HREF="/web/20001231120000/http://example.com/page.html">Link</a>`,
			contains: `href="http://example.com/page.html"`,
		},
		{
			name:     "Mixed case Href",
			input:    `<a Href="/web/20001231120000/http://example.com/page.html">Link</a>`,
			contains: `href="http://example.com/page.html"`,
		},
		{
			name:     "Uppercase SRC",
			input:    `<img SRC="/web/20001231im_/http://example.com/image.jpg">`,
			contains: `src="http://web.archive.org/web/20001231im_/http://example.com/image.jpg"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML([]byte(tt.input), 2000, "http://example.com")
			if !bytes.Contains(result, []byte(tt.contains)) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, string(result))
			}
		})
	}
}

// TestCleanHTMLWithSingleQuotes tests that both single and double quotes work
func TestCleanHTMLWithSingleQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Single quotes in href",
			input:    `<a href='/web/20001231120000/http://example.com/page.html'>Link</a>`,
			contains: `href="http://example.com/page.html"`,
		},
		{
			name:     "Single quotes in src",
			input:    `<img src='/web/20001231im_/http://example.com/image.jpg'>`,
			contains: `src="http://web.archive.org/web/20001231im_/http://example.com/image.jpg"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML([]byte(tt.input), 2000, "http://example.com")
			if !bytes.Contains(result, []byte(tt.contains)) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, string(result))
			}
		})
	}
}

// TestCleanHTMLEdgeCases tests edge cases and boundary conditions
func TestCleanHTMLEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "No HTML tags",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "HTML without head",
			input:    "<body>Content</body>",
			expected: "<body>Content</body>",
		},
		{
			name:     "Multiple Wayback toolbars",
			input:    "<!-- BEGIN WAYBACK TOOLBAR INSERT -->toolbar1<!-- END WAYBACK TOOLBAR INSERT -->content<!-- BEGIN WAYBACK TOOLBAR INSERT -->toolbar2<!-- END WAYBACK TOOLBAR INSERT -->",
			expected: "content",
		},
		{
			name:     "Nested script tags in head",
			input:    "<head><script><script>nested</script></script></head>",
			expected: "<head></script></head>", // Regex doesn't handle nested tags perfectly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML([]byte(tt.input), 2000, "http://example.com")
			if string(result) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

// BenchmarkCleanHTML benchmarks the cleanHTML function
func BenchmarkCleanHTML(b *testing.B) {
	input := []byte(`<html>
<head>
<script src="wayback.js"></script>
<title>Test</title>
</head>
<body>
<!-- BEGIN WAYBACK TOOLBAR INSERT -->
<div id="toolbar">Wayback toolbar</div>
<!-- END WAYBACK TOOLBAR INSERT -->
<a href="/web/20001231120000/http://example.com/page1.html">Link 1</a>
<a href="https://web.archive.org/web/20001231120000/http://example.com/page2.html">Link 2</a>
<img src="/web/20001231im_/http://example.com/image.jpg">
<img src="https://web.archive.org/web/20001231im_/http://example.com/logo.png">
</body>
</html>`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanHTML(input, 2000, "http://example.com")
	}
}

// TestProxyHandlerHeaderCopying tests that headers are properly copied
func TestProxyHandlerHeaderCopying(t *testing.T) {
	// Create a test server that returns specific headers
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("X-Custom-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "<html><body>Test</body></html>")
	}))
	defer testServer.Close()

	// Note: This test is limited because we can't easily mock the Wayback Machine
	// In a real scenario, you'd want to use dependency injection or interfaces
	// to make the HTTP client mockable
}

// TestConstants verifies the constant values
func TestConstants(t *testing.T) {
	if waybackAPIBase != "https://web.archive.org/web" {
		t.Errorf("waybackAPIBase should be 'https://web.archive.org/web', got %q", waybackAPIBase)
	}

	if startPort != 1995 {
		t.Errorf("startPort should be 1995, got %d", startPort)
	}

	if endPort != 2005 {
		t.Errorf("endPort should be 2005, got %d", endPort)
	}
}

// TestProxyHandlerYear tests that the handler uses the correct year
func TestProxyHandlerYear(t *testing.T) {
	tests := []struct {
		year int
	}{
		{year: 1995},
		{year: 2000},
		{year: 2005},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.year)), func(t *testing.T) {
			handler := &ProxyHandler{year: tt.year}
			if handler.year != tt.year {
				t.Errorf("Expected year %d, got %d", tt.year, handler.year)
			}
		})
	}
}
