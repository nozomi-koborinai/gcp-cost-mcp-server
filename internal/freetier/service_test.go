package freetier

import (
	"testing"
	"time"
)

func TestFindMatchingFreeTierItem(t *testing.T) {
	tests := []struct {
		name        string
		freeTier    *FreeTierInfo
		usageUnit   string
		expectMatch bool
		expectRes   string
	}{
		{
			name: "Match vCPU-seconds with 's' unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "vCPU-seconds", Amount: 240000, Unit: "seconds"},
				},
			},
			usageUnit:   "s",
			expectMatch: true,
			expectRes:   "vCPU-seconds",
		},
		{
			name: "Match GiB-seconds with 's' unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "GiB-seconds", Amount: 450000, Unit: "seconds"},
				},
			},
			usageUnit:   "s",
			expectMatch: true,
			expectRes:   "GiB-seconds",
		},
		{
			name: "Match storage with 'GiBy' unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "storage", Amount: 5, Unit: "GiB"},
				},
			},
			usageUnit:   "GiBy",
			expectMatch: true,
			expectRes:   "storage",
		},
		{
			name: "Match requests with 'count' unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "requests", Amount: 2000000, Unit: "count"},
				},
			},
			usageUnit:   "count",
			expectMatch: true,
			expectRes:   "requests",
		},
		{
			name: "Match operations with '1' unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "operations", Amount: 10000, Unit: "count"},
				},
			},
			usageUnit:   "1",
			expectMatch: true,
			expectRes:   "operations",
		},
		{
			name: "No match for unknown unit",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "vCPU-seconds", Amount: 240000, Unit: "seconds"},
				},
			},
			usageUnit:   "unknown",
			expectMatch: false,
		},
		{
			name:        "Nil freeTier",
			freeTier:    nil,
			usageUnit:   "s",
			expectMatch: false,
		},
		{
			name: "Empty items",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{},
			},
			usageUnit:   "s",
			expectMatch: false,
		},
		{
			name: "Multiple items - first match wins",
			freeTier: &FreeTierInfo{
				Items: []FreeTierItem{
					{Resource: "vCPU-seconds", Amount: 240000, Unit: "seconds"},
					{Resource: "GiB-seconds", Amount: 450000, Unit: "seconds"},
				},
			},
			usageUnit:   "s",
			expectMatch: true,
			expectRes:   "vCPU-seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindMatchingFreeTierItem(tt.freeTier, tt.usageUnit)

			if tt.expectMatch {
				if result == nil {
					t.Errorf("Expected match but got nil")
					return
				}
				if result.Resource != tt.expectRes {
					t.Errorf("Expected resource %q, got %q", tt.expectRes, result.Resource)
				}
			} else {
				if result != nil {
					t.Errorf("Expected no match but got %+v", result)
				}
			}
		})
	}
}

func TestCachedFreeTier_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "Not expired - future expiry",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  false,
		},
		{
			name:      "Expired - past expiry",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
		{
			name:      "Just expired - exact now",
			expiresAt: time.Now().Add(-1 * time.Millisecond),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cached := &CachedFreeTier{
				Info:      &FreeTierInfo{},
				CachedAt:  time.Now().Add(-1 * time.Hour),
				ExpiresAt: tt.expiresAt,
			}

			if cached.IsExpired() != tt.expected {
				t.Errorf("Expected IsExpired()=%v, got %v", tt.expected, cached.IsExpired())
			}
		})
	}
}

func TestNormalizeServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Cloud Run", "cloud-run"},
		{"cloud run", "cloud-run"},
		{"CLOUD RUN", "cloud-run"},
		{"  Cloud Run  ", "cloud-run"},
		{"BigQuery", "bigquery"},
		{"Cloud SQL", "cloud-sql"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeServiceName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeServiceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestService_CacheOperations(t *testing.T) {
	svc := NewService()

	// Initial cache should be empty
	stats := svc.GetCacheStats()
	if stats["total_entries"].(int) != 0 {
		t.Errorf("Expected empty cache, got %d entries", stats["total_entries"])
	}

	// Manually add a cache entry for testing
	svc.cacheMutex.Lock()
	svc.cache["test-service"] = &CachedFreeTier{
		Info: &FreeTierInfo{
			ServiceName: "Test Service",
			Items:       []FreeTierItem{{Resource: "test", Amount: 100, Unit: "count"}},
		},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	svc.cacheMutex.Unlock()

	// Check cache stats
	stats = svc.GetCacheStats()
	if stats["total_entries"].(int) != 1 {
		t.Errorf("Expected 1 cache entry, got %d", stats["total_entries"])
	}
	if stats["valid_entries"].(int) != 1 {
		t.Errorf("Expected 1 valid entry, got %d", stats["valid_entries"])
	}

	// Clear specific entry
	svc.ClearCacheEntry("Test Service")
	stats = svc.GetCacheStats()
	if stats["total_entries"].(int) != 0 {
		t.Errorf("Expected 0 cache entries after clear, got %d", stats["total_entries"])
	}

	// Add entry again and test ClearCache
	svc.cacheMutex.Lock()
	svc.cache["test-1"] = &CachedFreeTier{Info: &FreeTierInfo{}, CachedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	svc.cache["test-2"] = &CachedFreeTier{Info: &FreeTierInfo{}, CachedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	svc.cacheMutex.Unlock()

	svc.ClearCache()
	stats = svc.GetCacheStats()
	if stats["total_entries"].(int) != 0 {
		t.Errorf("Expected 0 cache entries after ClearCache, got %d", stats["total_entries"])
	}
}

func TestService_NewService(t *testing.T) {
	svc := NewService()

	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.searchClient == nil {
		t.Error("searchClient is nil")
	}
	if svc.scraperClient == nil {
		t.Error("scraperClient is nil")
	}
	if svc.cache == nil {
		t.Error("cache is nil")
	}
	if svc.cacheTTL != 24*time.Hour {
		t.Errorf("Expected cacheTTL 24h, got %v", svc.cacheTTL)
	}
}
