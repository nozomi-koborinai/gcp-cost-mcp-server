package freetier

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// GCPDocScraperClient scrapes GCP documentation pages for pricing information
type GCPDocScraperClient struct {
	httpClient *http.Client
}

// NewGCPDocScraperClient creates a new GCP documentation scraper client
func NewGCPDocScraperClient() *GCPDocScraperClient {
	return &GCPDocScraperClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchAsText fetches a GCP documentation page and returns the text content
func (c *GCPDocScraperClient) FetchAsText(ctx context.Context, url string) (string, error) {
	// Validate URL is from cloud.google.com
	if !strings.HasPrefix(url, "https://cloud.google.com") {
		return "", fmt.Errorf("URL must be from cloud.google.com")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic a browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GCP-Cost-MCP-Server/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("page returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse HTML and extract text content
	text, err := extractTextFromHTML(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to extract text: %w", err)
	}

	return text, nil
}

// extractTextFromHTML extracts readable text from HTML content
func extractTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	var extractText func(*html.Node)

	// Tags to skip
	skipTags := map[string]bool{
		"script":   true,
		"style":    true,
		"nav":      true,
		"header":   true,
		"footer":   true,
		"noscript": true,
		"svg":      true,
		"path":     true,
		"meta":     true,
		"link":     true,
	}

	extractText = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip certain tags
			if skipTags[n.Data] {
				return
			}

			// Focus on main content areas
			if n.Data == "main" || n.Data == "article" {
				// Process children
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					extractText(c)
				}
				return
			}
		}

		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				textBuilder.WriteString(text)
				textBuilder.WriteString(" ")
			}
		}

		// Process children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}

		// Add newlines after block elements
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "br":
				textBuilder.WriteString("\n")
			}
		}
	}

	extractText(doc)

	// Clean up the extracted text
	text := textBuilder.String()
	text = cleanText(text)

	return text, nil
}

// cleanText cleans up extracted text content
func cleanText(text string) string {
	// Remove excessive whitespace
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	// Remove excessive newlines
	newlineRegex := regexp.MustCompile(`\n{3,}`)
	text = newlineRegex.ReplaceAllString(text, "\n\n")

	// Trim
	text = strings.TrimSpace(text)

	return text
}

// ExtractPricingSection tries to extract specifically the pricing/free tier section
func (c *GCPDocScraperClient) ExtractPricingSection(content string) string {
	// Look for common pricing section indicators
	sectionMarkers := []string{
		"pricing",
		"free tier",
		"free usage",
		"at no charge",
		"no cost",
		"free of charge",
		"monthly free",
		"always free",
	}

	contentLower := strings.ToLower(content)

	// Find the start of pricing-related content
	var startIdx int = -1
	for _, marker := range sectionMarkers {
		idx := strings.Index(contentLower, marker)
		if idx != -1 && (startIdx == -1 || idx < startIdx) {
			startIdx = idx
		}
	}

	if startIdx == -1 {
		// No pricing section found, return full content
		return content
	}

	// Extract a reasonable chunk of content starting from the pricing section
	// Go back a bit to capture context
	startIdx = max(0, startIdx-200)

	// Take up to 5000 characters from that point
	endIdx := min(len(content), startIdx+5000)

	return content[startIdx:endIdx]
}

