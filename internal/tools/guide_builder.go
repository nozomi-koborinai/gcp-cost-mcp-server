package tools

import (
	"context"
	"fmt"
	"strings"
)

// analyzeSkusToGenerateGuide analyzes SKUs for a service and generates an estimation guide
func analyzeSkusToGenerateGuide(ctx context.Context, client PricingClient, serviceID, serviceName string) (*EstimationGuide, error) {
	// Fetch SKUs for the service
	resp, err := client.ListSKUs(ctx, serviceID, 500, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list SKUs: %w", err)
	}

	if len(resp.SKUs) == 0 {
		return nil, fmt.Errorf("no SKUs found for service")
	}

	// Analyze SKUs to extract information
	regions := make(map[string]bool)
	categories := make(map[string]bool)
	skuDescriptions := make([]string, 0)

	for _, sku := range resp.SKUs {
		// Collect regions
		if sku.GeoTaxonomy.RegionalMetadata.Region.Region != "" {
			regions[sku.GeoTaxonomy.RegionalMetadata.Region.Region] = true
		} else if sku.GeoTaxonomy.Type == "GLOBAL" {
			regions["global"] = true
		}

		// Collect categories
		for _, cat := range sku.ProductTaxonomy.TaxonomyCategories {
			categories[cat.Category] = true
		}

		// Collect SKU descriptions for analysis
		skuDescriptions = append(skuDescriptions, sku.DisplayName)
	}

	// Convert maps to slices
	regionList := mapKeysToSlice(regions)
	categoryList := mapKeysToSlice(categories)

	// Build parameters based on SKU analysis
	parameters := buildParametersFromSKUAnalysis(skuDescriptions, categoryList)

	// Build pricing factors from categories and SKU descriptions
	pricingFactors := buildPricingFactors(categoryList, skuDescriptions)

	// Build tips
	tips := buildTips(serviceName, categoryList)

	guide := &EstimationGuide{
		ServiceName:        serviceName,
		ServiceID:          serviceID,
		ServiceDescription: fmt.Sprintf("Google Cloud %s - pricing based on %d SKUs", serviceName, len(resp.SKUs)),
		Parameters:         parameters,
		PricingFactors:     pricingFactors,
		Tips:               tips,
		AvailableRegions:   regionList,
		SKUCategories:      categoryList,
	}

	return guide, nil
}

// skuParamRule maps keyword matches in SKU descriptions (and optionally
// categories) to a required parameter.
type skuParamRule struct {
	descKeywords []string
	catKeywords  []string
	param        RequiredParameter
}

// skuParamRules is evaluated in order; the order determines the parameter
// order in the generated guide.
var skuParamRules = []skuParamRule{
	{
		descKeywords: []string{"vcpu", "cpu", "core", "instance"},
		param: RequiredParameter{
			Name:        "vcpu_count",
			Description: "Number of vCPUs",
			Required:    true,
			Examples:    []string{"1", "2", "4", "8"},
			DefaultTip:  "More vCPUs = higher cost but better performance",
		},
	},
	{
		descKeywords: []string{"memory", "ram", "gib"},
		param: RequiredParameter{
			Name:        "memory_gib",
			Description: "Memory in GiB",
			Required:    true,
			Examples:    []string{"1", "2", "4", "8", "16"},
			DefaultTip:  "Memory is typically charged per GiB-hour or GiB-second",
		},
	},
	{
		descKeywords: []string{"storage", "disk", "ssd", "hdd", "persistent"},
		catKeywords:  []string{"storage"},
		param: RequiredParameter{
			Name:        "storage_gb",
			Description: "Storage capacity in GB",
			Required:    true,
			Examples:    []string{"10", "100", "500", "1000"},
			DefaultTip:  "Storage is typically charged per GB-month",
		},
	},
	{
		descKeywords: []string{"request", "invocation", "call", "api"},
		param: RequiredParameter{
			Name:        "requests_per_month",
			Description: "Expected number of requests per month",
			Required:    true,
			Examples:    []string{"10000", "100000", "1000000"},
			DefaultTip:  "Many services have free tiers for requests",
		},
	},
	{
		descKeywords: []string{"hour", "second", "minute", "time"},
		param: RequiredParameter{
			Name:        "monthly_hours",
			Description: "Expected running hours per month",
			Required:    true,
			Examples:    []string{"730 (24/7)", "176 (business hours)", "100"},
			DefaultTip:  "730 hours = full month of continuous operation",
		},
	},
	{
		descKeywords: []string{"egress", "network", "bandwidth", "transfer"},
		param: RequiredParameter{
			Name:        "egress_gb",
			Description: "Expected outbound data transfer in GB per month",
			Required:    false,
			Examples:    []string{"10", "100", "1000"},
			DefaultTip:  "Ingress is typically free, egress is charged",
		},
	},
	{
		descKeywords: []string{"instance", "node", "replica"},
		param: RequiredParameter{
			Name:        "instance_count",
			Description: "Number of instances or nodes",
			Required:    true,
			Examples:    []string{"1", "2", "3", "5"},
			DefaultTip:  "More instances = higher availability but higher cost",
		},
	},
}

// buildParametersFromSKUAnalysis builds required parameters based on SKU analysis
func buildParametersFromSKUAnalysis(skuDescriptions []string, categories []string) []RequiredParameter {
	params := []RequiredParameter{
		// Region is almost always required
		{
			Name:        "region",
			Description: "Deployment region or location",
			Required:    true,
			Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
			DefaultTip:  "Prices vary by region. Choose based on latency and compliance requirements.",
		},
	}

	descText := strings.ToLower(strings.Join(skuDescriptions, " "))
	catText := strings.ToLower(strings.Join(categories, " "))

	for _, rule := range skuParamRules {
		if containsAny(descText, rule.descKeywords) ||
			(len(rule.catKeywords) > 0 && containsAny(catText, rule.catKeywords)) {
			params = append(params, rule.param)
		}
	}

	return params
}

// buildPricingFactors generates pricing factors based on categories and SKU descriptions
func buildPricingFactors(categories []string, skuDescriptions []string) []string {
	factors := make([]string, 0)
	seen := make(map[string]bool)

	// Analyze categories
	for _, cat := range categories {
		catLower := strings.ToLower(cat)
		var factor string

		switch {
		case strings.Contains(catLower, "compute"):
			factor = "Compute time (vCPU-hours or vCPU-seconds)"
		case strings.Contains(catLower, "memory"):
			factor = "Memory usage (GiB-hours or GiB-seconds)"
		case strings.Contains(catLower, "storage"):
			factor = "Storage capacity (GB-month)"
		case strings.Contains(catLower, "network"):
			factor = "Network egress (per GB)"
		case strings.Contains(catLower, "request"):
			factor = "Request/API calls (per million)"
		}

		if factor != "" && !seen[factor] {
			factors = append(factors, factor)
			seen[factor] = true
		}
	}

	// Add generic factors if none found
	if len(factors) == 0 {
		factors = []string{
			"Usage-based pricing (check SKUs for specific units)",
			"Region-dependent pricing",
		}
	}

	return factors
}

// buildTips generates helpful tips for cost estimation
func buildTips(serviceName string, categories []string) []string {
	tips := []string{
		"Use list_services and list_skus to find specific SKU IDs for accurate pricing",
		"Regional pricing varies - check specific region costs",
	}

	// Add category-specific tips
	for _, cat := range categories {
		catLower := strings.ToLower(cat)
		if strings.Contains(catLower, "compute") {
			tips = append(tips, "Consider committed use discounts (CUDs) for sustained usage - up to 57% savings")
		}
		if strings.Contains(catLower, "storage") {
			tips = append(tips, "Use lifecycle policies to automatically move data to cheaper storage classes")
		}
	}

	tips = append(tips, "Free tier information is fetched from GCP documentation when available")

	return tips
}

// buildGenericGuide creates a generic estimation guide for unknown services
func buildGenericGuide(serviceName string) EstimationGuide {
	return EstimationGuide{
		ServiceName:        serviceName,
		ServiceDescription: "Dynamic guide not available - using generic GCP service estimation template",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "Deployment region or location",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1", "global"},
				DefaultTip:  "Prices vary significantly by region",
			},
			{
				Name:        "usage_type",
				Description: "How the service is billed (e.g., per hour, per request, per GB)",
				Required:    true,
				Examples:    []string{"time-based", "request-based", "storage-based", "data-processed"},
				DefaultTip:  "Use list_skus to discover billing units for this service",
			},
			{
				Name:        "expected_usage_amount",
				Description: "Expected usage quantity per month (in appropriate unit)",
				Required:    true,
				Examples:    []string{"730 hours", "1000000 requests", "100 GB"},
			},
			{
				Name:        "tier_or_edition",
				Description: "Service tier, edition, or configuration level",
				Required:    false,
				Examples:    []string{"Standard", "Enterprise", "Basic", "Premium"},
			},
		},
		PricingFactors: []string{
			"Compute/Processing time or capacity",
			"Storage capacity and class",
			"Data transfer (especially egress)",
			"Number of operations or requests",
		},
		Tips: []string{
			"IMPORTANT: Use list_services to find the service ID, then list_skus to discover available SKUs",
			"Check Google Cloud documentation for this service's specific pricing model",
			"Many services have free tiers - verify before estimating",
		},
	}
}

// buildSuggestedQuestion creates a question to ask the user based on the guide
func buildSuggestedQuestion(guide *EstimationGuide) string {
	var requiredParams []string
	for _, p := range guide.Parameters {
		if p.Required {
			requiredParams = append(requiredParams, p.Name)
		}
	}

	if len(requiredParams) == 0 {
		return fmt.Sprintf("To estimate %s costs, I need some usage details. What's your expected usage pattern?", guide.ServiceName)
	}

	question := fmt.Sprintf("To estimate %s costs accurately, I need to know: %s. Could you provide these details?",
		guide.ServiceName,
		strings.Join(requiredParams, ", "))

	// Add free tier note if available
	if guide.FreeTier != nil && guide.FreeTier.Available && len(guide.FreeTier.Items) > 0 {
		question += fmt.Sprintf("\n\nNote: %s has a free tier that will be automatically applied to the estimate.", guide.ServiceName)
	}

	return question
}

// Helper functions

func mapKeysToSlice(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func containsAny(text string, substrings []string) bool {
	for _, s := range substrings {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}
