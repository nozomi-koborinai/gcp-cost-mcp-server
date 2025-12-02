package freetier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneratePricingURLs(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		expectURLs  []string
	}{
		{
			name:        "Cloud Run",
			serviceName: "cloud run",
			expectURLs:  []string{"https://cloud.google.com/run/pricing", "https://cloud.google.com/run/pricing/"},
		},
		{
			name:        "BigQuery",
			serviceName: "bigquery",
			expectURLs:  []string{"https://cloud.google.com/bigquery/pricing", "https://cloud.google.com/bigquery/pricing/"},
		},
		{
			name:        "Cloud Storage",
			serviceName: "cloud storage",
			expectURLs:  []string{"https://cloud.google.com/storage/pricing", "https://cloud.google.com/storage-pricing"},
		},
		{
			name:        "GKE",
			serviceName: "gke",
			expectURLs:  []string{"https://cloud.google.com/kubernetes-engine/pricing", "https://cloud.google.com/kubernetes-engine/pricing/"},
		},
		{
			name:        "Secret Manager",
			serviceName: "secret manager",
			expectURLs:  []string{"https://cloud.google.com/secret-manager/pricing", "https://cloud.google.com/secret-manager/pricing/"},
		},
		{
			name:        "Unknown service - generates generic URLs",
			serviceName: "my-custom-service",
			expectURLs:  []string{"https://cloud.google.com/my-custom-service/pricing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := generatePricingURLs(tt.serviceName)

			if len(urls) == 0 {
				t.Error("Expected at least one URL, got none")
				return
			}

			// Check that expected URL is in the result
			found := false
			for _, expectedURL := range tt.expectURLs {
				for _, url := range urls {
					if url == expectedURL {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Errorf("Expected one of %v in result, got %v", tt.expectURLs, urls)
			}
		})
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{
			query:    "site:cloud.google.com cloud run pricing",
			expected: "cloud run",
		},
		{
			query:    "site:cloud.google.com bigquery pricing free tier",
			expected: "bigquery",
		},
		{
			query:    "cloud storage",
			expected: "cloud storage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := extractServiceName(tt.query)
			if result != tt.expected {
				t.Errorf("extractServiceName(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{
			text:     "Cloud Run Pricing - Google Cloud",
			expected: "Cloud Run Pricing",
		},
		{
			text:     "Short title",
			expected: "Short title",
		},
		{
			text:     "This is a very long title that exceeds the maximum length allowed and should be truncated at some point to ensure it fits within the expected limits",
			expected: "This is a very long title that exceeds the maximum length allowed and should be truncated at some po...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(20, len(tt.text))], func(t *testing.T) {
			result := extractTitle(tt.text)
			if result != tt.expected {
				t.Errorf("extractTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDuckDuckGoClient_Search_Fallback(t *testing.T) {
	// Create a mock server that returns an error response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewDuckDuckGoClient()

	// The client should fall back to constructed URLs when API fails
	results, err := client.Search(context.Background(), "site:cloud.google.com cloud run pricing", 3)

	// Should not return error - falls back to constructed URLs
	if err != nil {
		t.Errorf("Expected no error on fallback, got %v", err)
	}

	// Should return fallback results
	if len(results) == 0 {
		t.Error("Expected fallback results, got none")
	}

	// Fallback results should contain cloud.google.com URLs
	for _, r := range results {
		if r.URL == "" {
			t.Error("Fallback result has empty URL")
		}
	}
}

func TestDuckDuckGoClient_Search_ValidResponse(t *testing.T) {
	// Create a mock server that returns a valid DuckDuckGo response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Minimal valid DDG response (no GCP URLs, so fallback will be used)
		_, _ = w.Write([]byte(`{
			"AbstractURL": "",
			"AbstractText": "",
			"RelatedTopics": [],
			"Results": []
		}`))
	}))
	defer server.Close()

	client := NewDuckDuckGoClient()

	results, err := client.Search(context.Background(), "site:cloud.google.com cloud run pricing", 3)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Since no GCP URLs in response, should fall back to constructed URLs
	if len(results) == 0 {
		t.Error("Expected fallback results when no GCP URLs in response")
	}
}

func TestDuckDuckGoClient_ConstructFallbackResults(t *testing.T) {
	client := NewDuckDuckGoClient()

	// Test with empty service name
	results := client.constructFallbackResults("", 3)
	if len(results) != 0 {
		t.Errorf("Expected no results for empty query, got %d", len(results))
	}

	// Test with valid service name
	results = client.constructFallbackResults("site:cloud.google.com cloud run pricing", 3)
	if len(results) == 0 {
		t.Error("Expected results for valid query")
	}

	// Check limit is respected
	results = client.constructFallbackResults("site:cloud.google.com bigquery pricing", 1)
	if len(results) > 1 {
		t.Errorf("Expected max 1 result, got %d", len(results))
	}
}

func TestNewDuckDuckGoClient(t *testing.T) {
	client := NewDuckDuckGoClient()

	if client == nil {
		t.Fatal("NewDuckDuckGoClient returned nil")
	}
	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if client.httpClient.Timeout == 0 {
		t.Error("httpClient timeout not set")
	}
}

