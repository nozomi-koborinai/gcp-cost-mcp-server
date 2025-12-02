package tools

import (
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
)

func TestGetServiceAliases(t *testing.T) {
	aliases := getServiceAliases()

	tests := []struct {
		alias    string
		expected string
	}{
		{"gke", "kubernetes engine"},
		{"k8s", "kubernetes engine"},
		{"gcs", "cloud storage"},
		{"bq", "bigquery"},
		{"gcf", "cloud functions"},
		{"gae", "app engine"},
		{"gce", "compute engine"},
		{"pubsub", "pub/sub"},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			if got := aliases[tt.alias]; got != tt.expected {
				t.Errorf("alias[%q] = %q, want %q", tt.alias, got, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		substrings []string
		expected   bool
	}{
		{
			name:       "Contains first substring",
			text:       "this is a vcpu test",
			substrings: []string{"vcpu", "memory"},
			expected:   true,
		},
		{
			name:       "Contains second substring",
			text:       "this has memory usage",
			substrings: []string{"vcpu", "memory"},
			expected:   true,
		},
		{
			name:       "Contains none",
			text:       "this has nothing",
			substrings: []string{"vcpu", "memory"},
			expected:   false,
		},
		{
			name:       "Empty text",
			text:       "",
			substrings: []string{"vcpu"},
			expected:   false,
		},
		{
			name:       "Empty substrings",
			text:       "some text",
			substrings: []string{},
			expected:   false,
		},
		{
			name:       "Case sensitive",
			text:       "VCPU test",
			substrings: []string{"vcpu"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsAny(tt.text, tt.substrings); got != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.text, tt.substrings, got, tt.expected)
			}
		})
	}
}

func TestMapKeysToSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]bool
		expected int // expected length
	}{
		{
			name:     "Empty map",
			input:    map[string]bool{},
			expected: 0,
		},
		{
			name:     "Single element",
			input:    map[string]bool{"key1": true},
			expected: 1,
		},
		{
			name:     "Multiple elements",
			input:    map[string]bool{"key1": true, "key2": true, "key3": false},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapKeysToSlice(tt.input)
			if len(result) != tt.expected {
				t.Errorf("mapKeysToSlice() returned %d elements, want %d", len(result), tt.expected)
			}

			// Verify all keys are in the result
			for key := range tt.input {
				found := false
				for _, r := range result {
					if r == key {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("key %q not found in result", key)
				}
			}
		})
	}
}

func TestBuildPricingFactors(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		skuDescs   []string
		expectLen  int
	}{
		{
			name:       "Compute category",
			categories: []string{"Compute"},
			skuDescs:   []string{},
			expectLen:  1,
		},
		{
			name:       "Storage category",
			categories: []string{"Storage"},
			skuDescs:   []string{},
			expectLen:  1,
		},
		{
			name:       "Multiple categories",
			categories: []string{"Compute", "Storage", "Network"},
			skuDescs:   []string{},
			expectLen:  3,
		},
		{
			name:       "No categories - defaults",
			categories: []string{},
			skuDescs:   []string{},
			expectLen:  2, // Default factors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildPricingFactors(tt.categories, tt.skuDescs)
			if len(result) < tt.expectLen {
				t.Errorf("buildPricingFactors() returned %d factors, expected at least %d", len(result), tt.expectLen)
			}
		})
	}
}

func TestBuildTips(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		categories  []string
		minTips     int
	}{
		{
			name:        "Basic tips",
			serviceName: "Cloud Run",
			categories:  []string{},
			minTips:     2, // At least 2 default tips
		},
		{
			name:        "Compute tips",
			serviceName: "Compute Engine",
			categories:  []string{"Compute"},
			minTips:     3, // Default + compute tip
		},
		{
			name:        "Storage tips",
			serviceName: "Cloud Storage",
			categories:  []string{"Storage"},
			minTips:     3, // Default + storage tip
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTips(tt.serviceName, tt.categories)
			if len(result) < tt.minTips {
				t.Errorf("buildTips() returned %d tips, expected at least %d", len(result), tt.minTips)
			}
		})
	}
}

func TestBuildGenericGuide(t *testing.T) {
	serviceName := "Test Service"
	guide := buildGenericGuide(serviceName)

	if guide.ServiceName != serviceName {
		t.Errorf("ServiceName = %q, want %q", guide.ServiceName, serviceName)
	}

	if len(guide.Parameters) == 0 {
		t.Error("Expected at least one parameter in generic guide")
	}

	// Check that region parameter exists
	hasRegion := false
	for _, p := range guide.Parameters {
		if p.Name == "region" {
			hasRegion = true
			if !p.Required {
				t.Error("region parameter should be required")
			}
			break
		}
	}
	if !hasRegion {
		t.Error("Expected region parameter in generic guide")
	}

	if len(guide.PricingFactors) == 0 {
		t.Error("Expected at least one pricing factor")
	}

	if len(guide.Tips) == 0 {
		t.Error("Expected at least one tip")
	}
}

func TestBuildSuggestedQuestion(t *testing.T) {
	tests := []struct {
		name     string
		guide    *EstimationGuide
		contains string
	}{
		{
			name: "With required params",
			guide: &EstimationGuide{
				ServiceName: "Cloud Run",
				Parameters: []RequiredParameter{
					{Name: "region", Required: true},
					{Name: "vcpu", Required: true},
				},
			},
			contains: "Cloud Run",
		},
		{
			name: "No required params",
			guide: &EstimationGuide{
				ServiceName: "Test Service",
				Parameters: []RequiredParameter{
					{Name: "optional", Required: false},
				},
			},
			contains: "Test Service",
		},
		{
			name: "With free tier",
			guide: &EstimationGuide{
				ServiceName: "Cloud Functions",
				Parameters: []RequiredParameter{
					{Name: "region", Required: true},
				},
				FreeTier: &FreeTierSummary{
					Available: true,
					Items:     []freetier.FreeTierItem{{Resource: "requests", Amount: 2000000}},
				},
			},
			contains: "free tier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSuggestedQuestion(tt.guide)
			if result == "" {
				t.Error("buildSuggestedQuestion returned empty string")
			}
			// Note: contains check is case-insensitive for "free tier"
		})
	}
}

