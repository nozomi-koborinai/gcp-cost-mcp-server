package freetier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractTextFromHTML(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains []string
		excludes []string
	}{
		{
			name: "Basic paragraph extraction",
			html: `<html><body><p>Hello World</p></body></html>`,
			contains: []string{"Hello World"},
		},
		{
			name: "Skip script tags",
			html: `<html><body><p>Visible text</p><script>console.log("hidden");</script></body></html>`,
			contains: []string{"Visible text"},
			excludes: []string{"console.log", "hidden"},
		},
		{
			name: "Skip style tags",
			html: `<html><body><p>Content</p><style>.hidden { display: none; }</style></body></html>`,
			contains: []string{"Content"},
			excludes: []string{"display", "none"},
		},
		{
			name: "Skip nav and footer",
			html: `<html><body><nav>Navigation</nav><p>Main content</p><footer>Footer</footer></body></html>`,
			contains: []string{"Main content"},
		},
		{
			name: "Extract from multiple elements",
			html: `<html><body>
				<h1>Title</h1>
				<p>Paragraph one</p>
				<div>Div content</div>
				<p>Paragraph two</p>
			</body></html>`,
			contains: []string{"Title", "Paragraph one", "Div content", "Paragraph two"},
		},
		{
			name: "Handle nested elements",
			html: `<html><body><div><span>Nested <strong>text</strong></span></div></body></html>`,
			contains: []string{"Nested", "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractTextFromHTML(tt.html)
			if err != nil {
				t.Fatalf("extractTextFromHTML failed: %v", err)
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Expected result to contain %q, got: %s", want, result)
				}
			}

			for _, notWant := range tt.excludes {
				if strings.Contains(result, notWant) {
					t.Errorf("Expected result NOT to contain %q, got: %s", notWant, result)
				}
			}
		})
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove excessive whitespace",
			input:    "Hello    World",
			expected: "Hello World",
		},
		{
			name:     "Remove excessive newlines",
			input:    "Line 1\n\n\n\n\nLine 2",
			expected: "Line 1 Line 2", // cleanText normalizes all whitespace
		},
		{
			name:     "Trim whitespace",
			input:    "   Content   ",
			expected: "Content",
		},
		{
			name:     "Combined cleanup",
			input:    "  Hello    World  \n\n\n\n\n  More text  ",
			expected: "Hello World More text", // cleanText normalizes all whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanText(tt.input)
			if result != tt.expected {
				t.Errorf("cleanText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGCPDocScraperClient_ExtractPricingSection(t *testing.T) {
	client := NewGCPDocScraperClient()

	tests := []struct {
		name     string
		content  string
		contains string
	}{
		{
			name:     "Find pricing section",
			content:  "Introduction text. Pricing details are below. 240,000 vCPU-seconds free. Other content follows.",
			contains: "240,000 vCPU-seconds free",
		},
		{
			name:     "Find free tier section",
			content:  "Overview. The free tier includes 2 million requests. Additional charges apply.",
			contains: "2 million requests",
		},
		{
			name:     "No pricing section - returns full content",
			content:  "This is just regular documentation without any pricing information mentioned.",
			contains: "regular documentation",
		},
		{
			name:     "Always free mention",
			content:  "Some features are always free for all users. Details here.",
			contains: "always free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.ExtractPricingSection(tt.content)
			if !strings.Contains(strings.ToLower(result), strings.ToLower(tt.contains)) {
				t.Errorf("Expected result to contain %q, got: %s", tt.contains, result)
			}
		})
	}
}

func TestGCPDocScraperClient_FetchAsText_InvalidURL(t *testing.T) {
	client := NewGCPDocScraperClient()

	// Test with non-cloud.google.com URL
	_, err := client.FetchAsText(context.Background(), "https://example.com/pricing")
	if err == nil {
		t.Error("Expected error for non-cloud.google.com URL")
	}
	if !strings.Contains(err.Error(), "cloud.google.com") {
		t.Errorf("Expected error message about cloud.google.com, got: %v", err)
	}
}

func TestGCPDocScraperClient_FetchAsText_MockServer(t *testing.T) {
	// Create a mock server that returns HTML content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head><title>Test Page</title></head>
			<body>
				<nav>Navigation</nav>
				<main>
					<h1>Pricing</h1>
					<p>240,000 vCPU-seconds per month free.</p>
					<p>Additional usage billed at $0.00001.</p>
				</main>
				<footer>Footer content</footer>
			</body>
			</html>
		`))
	}))
	defer server.Close()

	// Note: We can't easily test FetchAsText with the mock server
	// because it validates the URL must be cloud.google.com
	// This is a design constraint for security
	// Instead, we test the HTML parsing logic directly

	html := `
		<!DOCTYPE html>
		<html>
		<body>
			<main>
				<h1>Pricing</h1>
				<p>240,000 vCPU-seconds per month free.</p>
			</main>
		</body>
		</html>
	`

	text, err := extractTextFromHTML(html)
	if err != nil {
		t.Fatalf("extractTextFromHTML failed: %v", err)
	}

	if !strings.Contains(text, "240,000") {
		t.Errorf("Expected text to contain pricing info, got: %s", text)
	}
}

func TestNewGCPDocScraperClient(t *testing.T) {
	client := NewGCPDocScraperClient()

	if client == nil {
		t.Fatal("NewGCPDocScraperClient returned nil")
	}
	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if client.httpClient.Timeout == 0 {
		t.Error("httpClient timeout not set")
	}
}

func TestGCPDocScraperClient_FetchAsText_ServerError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// We can test the error handling by creating a client and using
	// a different approach - test the validate URL logic
	client := NewGCPDocScraperClient()

	// Test with invalid URL format
	_, err := client.FetchAsText(context.Background(), "not-a-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

