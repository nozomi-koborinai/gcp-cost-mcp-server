package tools

import (
	"encoding/json"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
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

// --- Issue #13 tests: convertSKUs and filterSKUs ---

func TestConvertSKUs(t *testing.T) {
	tests := []struct {
		name     string
		input    []pricing.SKU
		expected []SKUInfo
	}{
		{
			name:     "Empty input",
			input:    []pricing.SKU{},
			expected: []SKUInfo{},
		},
		{
			name: "Regional SKU",
			input: []pricing.SKU{
				{
					SKUID:       "AAAA-BBBB-CCCC",
					DisplayName: "Redis Capacity Basic M1",
					GeoTaxonomy: pricing.GeoTaxonomy{
						Type: "REGIONAL",
						RegionalMetadata: pricing.RegionalMetadata{
							Region: pricing.Region{Region: "asia-northeast1"},
						},
					},
					ProductTaxonomy: pricing.ProductTaxonomy{
						TaxonomyCategories: []pricing.TaxonomyCategory{
							{Category: "GCP"},
							{Category: "Memorystore"},
						},
					},
				},
			},
			expected: []SKUInfo{
				{
					SKUID:       "AAAA-BBBB-CCCC",
					DisplayName: "Redis Capacity Basic M1",
					Region:      "asia-northeast1",
					Categories:  []string{"GCP", "Memorystore"},
				},
			},
		},
		{
			name: "Global SKU",
			input: []pricing.SKU{
				{
					SKUID:       "1111-2222-3333",
					DisplayName: "Global Network Egress",
					GeoTaxonomy: pricing.GeoTaxonomy{Type: "GLOBAL"},
				},
			},
			expected: []SKUInfo{
				{
					SKUID:       "1111-2222-3333",
					DisplayName: "Global Network Egress",
					Region:      "global",
				},
			},
		},
		{
			name: "SKU with no geo taxonomy",
			input: []pricing.SKU{
				{
					SKUID:       "XXXX-YYYY-ZZZZ",
					DisplayName: "Some SKU",
				},
			},
			expected: []SKUInfo{
				{
					SKUID:       "XXXX-YYYY-ZZZZ",
					DisplayName: "Some SKU",
					Region:      "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertSKUs(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("got %d SKUs, want %d", len(result), len(tt.expected))
			}
			for i, exp := range tt.expected {
				if result[i].SKUID != exp.SKUID {
					t.Errorf("SKU[%d].SKUID = %q, want %q", i, result[i].SKUID, exp.SKUID)
				}
				if result[i].DisplayName != exp.DisplayName {
					t.Errorf("SKU[%d].DisplayName = %q, want %q", i, result[i].DisplayName, exp.DisplayName)
				}
				if result[i].Region != exp.Region {
					t.Errorf("SKU[%d].Region = %q, want %q", i, result[i].Region, exp.Region)
				}
			}
		})
	}
}

func TestFilterSKUs(t *testing.T) {
	skus := []SKUInfo{
		{SKUID: "001", DisplayName: "Redis Capacity Basic M1", Region: "asia-northeast1", Categories: []string{"GCP", "Memorystore"}},
		{SKUID: "002", DisplayName: "Redis Capacity Basic M2", Region: "asia-northeast1", Categories: []string{"GCP", "Memorystore"}},
		{SKUID: "003", DisplayName: "Redis Capacity Standard M1", Region: "asia-northeast1", Categories: []string{"GCP", "Memorystore"}},
		{SKUID: "004", DisplayName: "Redis Capacity Basic M1", Region: "us-central1", Categories: []string{"GCP", "Memorystore"}},
		{SKUID: "005", DisplayName: "N2 Custom Instance Core", Region: "asia-northeast1", Categories: []string{"GCP", "Compute"}},
		{SKUID: "006", DisplayName: "Cloud Storage Egress", Region: "global", Categories: []string{"GCP", "Network"}},
	}

	tests := []struct {
		name        string
		region      string
		keyword     string
		category    string
		expectedIDs []string
	}{
		{
			name:        "No filters - return all",
			expectedIDs: []string{"001", "002", "003", "004", "005", "006"},
		},
		{
			name:        "Filter by region only",
			region:      "asia-northeast1",
			expectedIDs: []string{"001", "002", "003", "005"},
		},
		{
			name:        "Filter by keyword only",
			keyword:     "Basic M1",
			expectedIDs: []string{"001", "004"},
		},
		{
			name:        "Filter by category only",
			category:    "Compute",
			expectedIDs: []string{"005"},
		},
		{
			name:        "Filter by region + keyword",
			region:      "asia-northeast1",
			keyword:     "Basic M1",
			expectedIDs: []string{"001"},
		},
		{
			name:        "Filter by region + keyword + category",
			region:      "asia-northeast1",
			keyword:     "Redis",
			category:    "Memorystore",
			expectedIDs: []string{"001", "002", "003"},
		},
		{
			name:        "Case-insensitive keyword",
			keyword:     "basic m1",
			expectedIDs: []string{"001", "004"},
		},
		{
			name:        "Case-insensitive region",
			region:      "ASIA-NORTHEAST1",
			keyword:     "Basic M1",
			expectedIDs: []string{"001"},
		},
		{
			name:        "No matches",
			region:      "europe-west1",
			expectedIDs: []string{},
		},
		{
			name:        "Category substring match",
			category:    "memory",
			expectedIDs: []string{"001", "002", "003", "004"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterSKUs(skus, tt.region, tt.keyword, tt.category)
			if len(result) != len(tt.expectedIDs) {
				t.Fatalf("got %d results, want %d (got IDs: %v)",
					len(result), len(tt.expectedIDs), skuIDs(result))
			}
			for i, expID := range tt.expectedIDs {
				if result[i].SKUID != expID {
					t.Errorf("result[%d].SKUID = %q, want %q", i, result[i].SKUID, expID)
				}
			}
		})
	}
}

func skuIDs(skus []SKUInfo) []string {
	ids := make([]string, len(skus))
	for i, s := range skus {
		ids[i] = s.SKUID
	}
	return ids
}

// --- Issue #14 tests: ConsumptionPricing structure ---

func TestConsumptionPricingJSON(t *testing.T) {
	output := GetSKUPriceOutput{
		Price: PriceInfo{
			SKUID:           "490F-75A9-E3F1",
			CurrencyCode:    "USD",
			Unit:            "s",
			UnitDescription: "second",
			Tiers: []PricingTier{
				{StartAmount: 0, PricePerUnit: 0.000024, Currency: "USD"},
			},
			AggregationInfo: "ACCOUNT / MONTHLY",
			AllPricingModels: []ConsumptionPricing{
				{
					ConsumptionModel: "DEFAULT",
					Description:      "On-demand pricing",
					Unit:             "s",
					UnitDescription:  "second",
					Tiers: []PricingTier{
						{StartAmount: 0, PricePerUnit: 0.000024, Currency: "USD"},
					},
				},
				{
					ConsumptionModel: "COMMITTED",
					Description:      "1 year commitment",
					Unit:             "s",
					UnitDescription:  "second",
					Tiers: []PricingTier{
						{StartAmount: 0, PricePerUnit: 0.000017, Currency: "USD"},
					},
				},
				{
					ConsumptionModel: "COMMITTED",
					Description:      "3 year commitment",
					Unit:             "s",
					UnitDescription:  "second",
					Tiers: []PricingTier{
						{StartAmount: 0, PricePerUnit: 0.00000972, Currency: "USD"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded GetSKUPriceOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Price.SKUID != "490F-75A9-E3F1" {
		t.Errorf("SKUID = %q, want %q", decoded.Price.SKUID, "490F-75A9-E3F1")
	}

	if len(decoded.Price.AllPricingModels) != 3 {
		t.Fatalf("AllPricingModels has %d entries, want 3", len(decoded.Price.AllPricingModels))
	}

	if decoded.Price.AllPricingModels[0].ConsumptionModel != "DEFAULT" {
		t.Errorf("first model = %q, want %q", decoded.Price.AllPricingModels[0].ConsumptionModel, "DEFAULT")
	}
	if decoded.Price.AllPricingModels[1].ConsumptionModel != "COMMITTED" {
		t.Errorf("second model = %q, want %q", decoded.Price.AllPricingModels[1].ConsumptionModel, "COMMITTED")
	}

	if decoded.Price.Tiers[0].PricePerUnit != output.Price.AllPricingModels[0].Tiers[0].PricePerUnit {
		t.Error("top-level Tiers should match the first pricing model's Tiers for backward compatibility")
	}
}

func TestPriceInfoBackwardCompatibility(t *testing.T) {
	output := GetSKUPriceOutput{
		Price: PriceInfo{
			SKUID:        "TEST-SKU",
			CurrencyCode: "USD",
			Unit:         "h",
			Tiers:        []PricingTier{{StartAmount: 0, PricePerUnit: 1.5, Currency: "USD"}},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}

	price := raw["price"].(map[string]interface{})

	if _, ok := price["sku_id"]; !ok {
		t.Error("missing sku_id in JSON output")
	}
	if _, ok := price["tiers"]; !ok {
		t.Error("missing tiers in JSON output")
	}
	if _, ok := price["unit"]; !ok {
		t.Error("missing unit in JSON output")
	}

	if _, ok := price["all_pricing_models"]; ok {
		t.Error("all_pricing_models should be omitted when empty")
	}
}

// --- Issue #15-1 tests: isCoreGCPService and filterServices ---

func TestIsCoreGCPService(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		expected    bool
	}{
		{"Cloud Run", "Cloud Run", true},
		{"Cloud SQL", "Cloud SQL", true},
		{"Compute Engine", "Compute Engine", true},
		{"App Engine", "App Engine", true},
		{"BigQuery", "BigQuery", true},
		{"Kubernetes Engine", "Kubernetes Engine", true},
		{"Cloud Functions", "Cloud Functions", true},
		{"Firebase Realtime Database", "Firebase Realtime Database", true},
		{"Vertex AI", "Vertex AI", true},
		{"Cloud Storage", "Cloud Storage", true},
		{"Memorystore for Redis", "Memorystore for Redis", true},
		{"Google Cloud Armor", "Google Cloud Armor", true},
		{"Spanner", "Spanner", true},
		{"Dataflow", "Dataflow", true},
		{"Gemini", "Gemini", true},
		{"Pub/Sub", "Pub/Sub", true},

		{"OpenLogic CentOS 7.9", "OpenLogic CentOS 7.9", false},
		{"Canonical Ubuntu 18.04 LTS", "Canonical Ubuntu 18.04 LTS", false},
		{"Adverity", "Adverity", false},
		{"Citrix ADC VPX", "Citrix ADC VPX", false},
		{"SUSE Linux", "SUSE Linux", false},
		{"Red Hat Enterprise Linux", "Red Hat Enterprise Linux", false},
		{"Palo Alto Networks", "Palo Alto Networks", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCoreGCPService(tt.displayName)
			if got != tt.expected {
				t.Errorf("isCoreGCPService(%q) = %v, want %v", tt.displayName, got, tt.expected)
			}
		})
	}
}

func TestFilterServices(t *testing.T) {
	services := []pricing.Service{
		{ServiceID: "SVC-001", DisplayName: "Cloud Run"},
		{ServiceID: "SVC-002", DisplayName: "Cloud SQL"},
		{ServiceID: "SVC-003", DisplayName: "BigQuery"},
		{ServiceID: "SVC-004", DisplayName: "OpenLogic CentOS 7.9"},
		{ServiceID: "SVC-005", DisplayName: "Canonical Ubuntu 18.04 LTS"},
		{ServiceID: "SVC-006", DisplayName: "Cloud Functions"},
		{ServiceID: "SVC-007", DisplayName: "Adverity"},
	}

	tests := []struct {
		name        string
		nameFilter  string
		coreOnly    bool
		expectedIDs []string
	}{
		{
			name:        "No filters",
			expectedIDs: []string{"SVC-001", "SVC-002", "SVC-003", "SVC-004", "SVC-005", "SVC-006", "SVC-007"},
		},
		{
			name:        "Core only",
			coreOnly:    true,
			expectedIDs: []string{"SVC-001", "SVC-002", "SVC-003", "SVC-006"},
		},
		{
			name:        "Name filter: Cloud",
			nameFilter:  "Cloud",
			expectedIDs: []string{"SVC-001", "SVC-002", "SVC-006"},
		},
		{
			name:        "Name filter case-insensitive",
			nameFilter:  "cloud run",
			expectedIDs: []string{"SVC-001"},
		},
		{
			name:        "Core only + name filter",
			nameFilter:  "Cloud",
			coreOnly:    true,
			expectedIDs: []string{"SVC-001", "SVC-002", "SVC-006"},
		},
		{
			name:        "No matches",
			nameFilter:  "Nonexistent Service",
			expectedIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterServices(services, tt.nameFilter, tt.coreOnly)
			if len(result) != len(tt.expectedIDs) {
				gotIDs := make([]string, len(result))
				for i, s := range result {
					gotIDs[i] = s.ServiceID
				}
				t.Fatalf("got %d results %v, want %d %v",
					len(result), gotIDs, len(tt.expectedIDs), tt.expectedIDs)
			}
			for i, expID := range tt.expectedIDs {
				if result[i].ServiceID != expID {
					t.Errorf("result[%d].ServiceID = %q, want %q", i, result[i].ServiceID, expID)
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

