package freetier

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// FreeTierItem represents a single free tier resource allocation
type FreeTierItem struct {
	Resource string  `json:"resource"`
	Amount   float64 `json:"amount"`
	Unit     string  `json:"unit"`
}

// FreeTierInfo contains all free tier information for a service
type FreeTierInfo struct {
	ServiceName string         `json:"service_name"`
	Items       []FreeTierItem `json:"items"`
	Scope       string         `json:"scope"`  // "account" or "project"
	Period      string         `json:"period"` // "month", "day", or "always"
	Conditions  []string       `json:"conditions,omitempty"`
	SourceURL   string         `json:"source_url"`
}

// CachedFreeTier wraps FreeTierInfo with cache metadata
type CachedFreeTier struct {
	Info      *FreeTierInfo
	CachedAt  time.Time
	ExpiresAt time.Time
}

// IsExpired checks if the cached entry has expired
func (c *CachedFreeTier) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// Service provides free tier information retrieval with caching
type Service struct {
	searchClient  *DuckDuckGoClient
	scraperClient *GCPDocScraperClient
	cache         map[string]*CachedFreeTier
	cacheMutex    sync.RWMutex
	cacheTTL      time.Duration
}

// NewService creates a new FreeTierService
func NewService() *Service {
	return &Service{
		searchClient:  NewDuckDuckGoClient(),
		scraperClient: NewGCPDocScraperClient(),
		cache:         make(map[string]*CachedFreeTier),
		cacheTTL:      24 * time.Hour,
	}
}

// GetFreeTier retrieves free tier information for a GCP service
func (s *Service) GetFreeTier(ctx context.Context, serviceName string) (*FreeTierInfo, error) {
	// Normalize service name for cache key
	cacheKey := normalizeServiceName(serviceName)

	// Check cache first
	s.cacheMutex.RLock()
	cached, exists := s.cache[cacheKey]
	s.cacheMutex.RUnlock()

	if exists && !cached.IsExpired() {
		log.Printf("FreeTierService: Cache hit for %s", serviceName)
		return cached.Info, nil
	}

	log.Printf("FreeTierService: Cache miss for %s, fetching from documentation", serviceName)

	// Search for pricing page
	freeTier, err := s.fetchFreeTierFromDocs(ctx, serviceName)
	if err != nil {
		log.Printf("FreeTierService: Error fetching free tier for %s: %v", serviceName, err)
		// Don't fail completely, return nil with no error
		// This allows cost estimation to proceed without free tier info
		return nil, nil
	}

	// Cache the result
	s.cacheMutex.Lock()
	s.cache[cacheKey] = &CachedFreeTier{
		Info:      freeTier,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(s.cacheTTL),
	}
	s.cacheMutex.Unlock()

	return freeTier, nil
}

// fetchFreeTierFromDocs searches for and extracts free tier info from GCP documentation
func (s *Service) fetchFreeTierFromDocs(ctx context.Context, serviceName string) (*FreeTierInfo, error) {
	// Build search query
	query := fmt.Sprintf("site:cloud.google.com %s pricing", serviceName)

	// Search for pricing pages
	results, err := s.searchClient.Search(ctx, query, 3)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no pricing pages found for %s", serviceName)
	}

	// Try to fetch and extract from each result
	var lastErr error
	for _, result := range results {
		// Only process cloud.google.com URLs
		if !strings.Contains(result.URL, "cloud.google.com") {
			continue
		}

		// Skip non-pricing pages
		if !strings.Contains(strings.ToLower(result.URL), "pricing") &&
			!strings.Contains(strings.ToLower(result.Title), "pricing") {
			continue
		}

		log.Printf("FreeTierService: Fetching %s", result.URL)

		// Fetch the page content
		content, err := s.scraperClient.FetchAsText(ctx, result.URL)
		if err != nil {
			lastErr = err
			continue
		}

		// Extract pricing section
		pricingContent := s.scraperClient.ExtractPricingSection(content)

		// Extract free tier items
		items := ExtractFreeTierItems(pricingContent)
		if len(items) == 0 {
			// Try with full content
			items = ExtractFreeTierItems(content)
		}

		if len(items) > 0 {
			return &FreeTierInfo{
				ServiceName: serviceName,
				Items:       items,
				Scope:       ExtractScope(content),
				Period:      ExtractPeriod(content),
				SourceURL:   result.URL,
			}, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("no free tier information found for %s", serviceName)
}

// normalizeServiceName normalizes a service name for use as a cache key
func normalizeServiceName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// ClearCache clears the entire cache
func (s *Service) ClearCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache = make(map[string]*CachedFreeTier)
}

// ClearCacheEntry removes a specific entry from the cache
func (s *Service) ClearCacheEntry(serviceName string) {
	cacheKey := normalizeServiceName(serviceName)
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	delete(s.cache, cacheKey)
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() map[string]interface{} {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	validEntries := 0
	expiredEntries := 0

	for _, entry := range s.cache {
		if entry.IsExpired() {
			expiredEntries++
		} else {
			validEntries++
		}
	}

	return map[string]interface{}{
		"total_entries":   len(s.cache),
		"valid_entries":   validEntries,
		"expired_entries": expiredEntries,
		"ttl_hours":       s.cacheTTL.Hours(),
	}
}

// FindMatchingFreeTierItem finds a free tier item that matches the given usage unit
func FindMatchingFreeTierItem(freeTier *FreeTierInfo, usageUnit string) *FreeTierItem {
	if freeTier == nil || len(freeTier.Items) == 0 {
		return nil
	}

	// Map SKU usage units to free tier resource names
	unitMapping := map[string][]string{
		"s":     {"vCPU-seconds", "GiB-seconds", "seconds"},
		"GiBy":  {"GiB-seconds", "storage"},
		"GiBy.s": {"GiB-seconds"},
		"By":    {"storage", "egress"},
		"count": {"requests", "operations", "access-operations", "document-reads", "document-writes", "document-deletes"},
		"1":     {"requests", "operations", "secret-versions"},
		"h":     {"hours"},
		"mo":    {"months"},
	}

	possibleResources, ok := unitMapping[usageUnit]
	if !ok {
		// Try to find a direct match
		for i := range freeTier.Items {
			if strings.EqualFold(freeTier.Items[i].Unit, usageUnit) {
				return &freeTier.Items[i]
			}
		}
		return nil
	}

	// Look for matching free tier item
	for i := range freeTier.Items {
		for _, resource := range possibleResources {
			if strings.Contains(strings.ToLower(freeTier.Items[i].Resource), strings.ToLower(resource)) {
				return &freeTier.Items[i]
			}
		}
	}

	return nil
}

