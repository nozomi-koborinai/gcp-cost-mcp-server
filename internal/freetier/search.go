package freetier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SearchResult represents a single search result
type SearchResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoClient provides search functionality using DuckDuckGo
type DuckDuckGoClient struct {
	httpClient *http.Client
}

// NewDuckDuckGoClient creates a new DuckDuckGo search client
func NewDuckDuckGoClient() *DuckDuckGoClient {
	return &DuckDuckGoClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// duckDuckGoResponse represents the DuckDuckGo API response
type duckDuckGoResponse struct {
	RelatedTopics []struct {
		FirstURL string `json:"FirstURL"`
		Text     string `json:"Text"`
	} `json:"RelatedTopics"`
	Results []struct {
		FirstURL string `json:"FirstURL"`
		Text     string `json:"Text"`
	} `json:"Results"`
	AbstractURL  string `json:"AbstractURL"`
	AbstractText string `json:"AbstractText"`
}

// Search performs a search using DuckDuckGo Instant Answers API
// Note: DuckDuckGo's free API is limited, so we construct likely URLs as fallback
func (c *DuckDuckGoClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// Build the DuckDuckGo API URL
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("no_redirect", "1")
	params.Set("skip_disambig", "1")

	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?%s", params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "GCP-Cost-MCP-Server/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// If API fails, return constructed URLs as fallback
		return c.constructFallbackResults(query, limit), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.constructFallbackResults(query, limit), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.constructFallbackResults(query, limit), nil
	}

	var ddgResp duckDuckGoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return c.constructFallbackResults(query, limit), nil
	}

	var results []SearchResult

	// Add abstract URL if available and relevant
	if ddgResp.AbstractURL != "" && strings.Contains(ddgResp.AbstractURL, "cloud.google.com") {
		results = append(results, SearchResult{
			URL:     ddgResp.AbstractURL,
			Title:   "GCP Documentation",
			Snippet: ddgResp.AbstractText,
		})
	}

	// Add related topics
	for _, topic := range ddgResp.RelatedTopics {
		if topic.FirstURL != "" && strings.Contains(topic.FirstURL, "cloud.google.com") {
			results = append(results, SearchResult{
				URL:     topic.FirstURL,
				Title:   extractTitle(topic.Text),
				Snippet: topic.Text,
			})
		}
		if len(results) >= limit {
			break
		}
	}

	// If no results from API, use fallback
	if len(results) == 0 {
		return c.constructFallbackResults(query, limit), nil
	}

	return results, nil
}

// constructFallbackResults creates likely GCP documentation URLs based on service name
func (c *DuckDuckGoClient) constructFallbackResults(query string, limit int) []SearchResult {
	// Extract service name from query
	serviceName := extractServiceName(query)
	if serviceName == "" {
		return nil
	}

	// Generate likely pricing page URLs
	baseURLs := generatePricingURLs(serviceName)

	var results []SearchResult
	for i, url := range baseURLs {
		if i >= limit {
			break
		}
		results = append(results, SearchResult{
			URL:     url,
			Title:   fmt.Sprintf("%s Pricing - Google Cloud", serviceName),
			Snippet: "Pricing information for " + serviceName,
		})
	}

	return results
}

// extractTitle extracts a title from DuckDuckGo text response
func extractTitle(text string) string {
	// DuckDuckGo text often has format "Title - Description"
	if idx := strings.Index(text, " - "); idx > 0 && idx < 100 {
		return text[:idx]
	}
	if len(text) > 100 {
		return text[:100] + "..."
	}
	return text
}

// extractServiceName extracts the GCP service name from a search query
func extractServiceName(query string) string {
	// Remove common search terms
	query = strings.ToLower(query)
	query = strings.ReplaceAll(query, "site:cloud.google.com", "")
	query = strings.ReplaceAll(query, "pricing", "")
	query = strings.ReplaceAll(query, "free tier", "")
	query = strings.TrimSpace(query)
	return query
}

// generatePricingURLs generates likely GCP pricing page URLs for a service
func generatePricingURLs(serviceName string) []string {
	// Normalize service name for URL
	urlName := strings.ToLower(serviceName)
	urlName = strings.ReplaceAll(urlName, " ", "-")
	urlName = strings.ReplaceAll(urlName, "_", "-")

	// Map common service names to their URL paths
	urlMappings := map[string][]string{
		"cloud run":           {"run/pricing", "run/pricing/"},
		"cloud-run":           {"run/pricing", "run/pricing/"},
		"compute engine":      {"compute/all-pricing", "compute/pricing"},
		"compute-engine":      {"compute/all-pricing", "compute/pricing"},
		"cloud storage":       {"storage/pricing", "storage-pricing"},
		"cloud-storage":       {"storage/pricing", "storage-pricing"},
		"bigquery":            {"bigquery/pricing", "bigquery/pricing/"},
		"cloud sql":           {"sql/pricing", "sql/pricing/"},
		"cloud-sql":           {"sql/pricing", "sql/pricing/"},
		"gke":                 {"kubernetes-engine/pricing", "kubernetes-engine/pricing/"},
		"kubernetes engine":   {"kubernetes-engine/pricing", "kubernetes-engine/pricing/"},
		"cloud functions":     {"functions/pricing", "functions/pricing/"},
		"cloud-functions":     {"functions/pricing", "functions/pricing/"},
		"pub/sub":             {"pubsub/pricing", "pubsub/pricing/"},
		"pubsub":              {"pubsub/pricing", "pubsub/pricing/"},
		"firestore":           {"firestore/pricing", "firestore/pricing/"},
		"spanner":             {"spanner/pricing", "spanner/pricing/"},
		"cloud spanner":       {"spanner/pricing", "spanner/pricing/"},
		"memorystore":         {"memorystore/pricing", "memorystore/pricing/"},
		"cloud cdn":           {"cdn/pricing", "cdn/pricing/"},
		"cloud armor":         {"armor/pricing", "armor/pricing/"},
		"artifact registry":   {"artifact-registry/pricing", "artifact-registry/pricing/"},
		"artifact-registry":   {"artifact-registry/pricing", "artifact-registry/pricing/"},
		"secret manager":      {"secret-manager/pricing", "secret-manager/pricing/"},
		"secret-manager":      {"secret-manager/pricing", "secret-manager/pricing/"},
		"app engine":          {"appengine/pricing", "appengine/pricing/"},
		"app-engine":          {"appengine/pricing", "appengine/pricing/"},
		"cloud load balancing": {"load-balancing/pricing", "load-balancing/pricing/"},
		"vertex ai":           {"vertex-ai/pricing", "vertex-ai/pricing/"},
		"vertex-ai":           {"vertex-ai/pricing", "vertex-ai/pricing/"},
	}

	baseURL := "https://cloud.google.com/"

	// Check for known mappings
	if paths, ok := urlMappings[urlName]; ok {
		var urls []string
		for _, path := range paths {
			urls = append(urls, baseURL+path)
		}
		return urls
	}

	// Generate generic URLs for unknown services
	return []string{
		baseURL + urlName + "/pricing",
		baseURL + urlName + "-pricing",
		baseURL + strings.ReplaceAll(urlName, "-", "/") + "/pricing",
	}
}

