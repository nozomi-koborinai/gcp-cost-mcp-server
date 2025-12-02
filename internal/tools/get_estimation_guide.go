package tools

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// GetEstimationGuideInput is the input for the get_estimation_guide tool
type GetEstimationGuideInput struct {
	ServiceName string `json:"service_name" jsonschema_description:"The Google Cloud service name to get estimation requirements for. Works with ANY GCP service - the tool dynamically generates guides based on SKU analysis. Examples: 'Cloud Run', 'BigQuery', 'Vertex AI', 'Cloud Logging', 'Dataflow', etc."`
}

// RequiredParameter represents a parameter needed for cost estimation
type RequiredParameter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Examples    []string `json:"examples,omitempty"`
	DefaultTip  string   `json:"default_tip,omitempty"`
}

// FreeTierSummary represents free tier information in the guide
type FreeTierSummary struct {
	Available bool                    `json:"available"`
	Items     []freetier.FreeTierItem `json:"items,omitempty"`
	Scope     string                  `json:"scope,omitempty"`
	Period    string                  `json:"period,omitempty"`
	SourceURL string                  `json:"source_url,omitempty"`
}

// EstimationGuide represents the guide for estimating costs
type EstimationGuide struct {
	ServiceName        string              `json:"service_name"`
	ServiceID          string              `json:"service_id,omitempty"`
	ServiceDescription string              `json:"service_description"`
	Parameters         []RequiredParameter `json:"parameters"`
	PricingFactors     []string            `json:"pricing_factors"`
	Tips               []string            `json:"tips,omitempty"`
	AvailableRegions   []string            `json:"available_regions,omitempty"`
	FreeTier           *FreeTierSummary    `json:"free_tier,omitempty"`
	SKUCategories      []string            `json:"sku_categories,omitempty"`
}

// GetEstimationGuideOutput is the output of the get_estimation_guide tool
type GetEstimationGuideOutput struct {
	Guide             EstimationGuide `json:"guide"`
	SuggestedQuestion string          `json:"suggested_question"`
}

// NewGetEstimationGuide creates a tool that provides estimation requirements for GCP services
func NewGetEstimationGuide(g *genkit.Genkit, pricingClient *pricing.Client, freeTierService *freetier.Service) ai.Tool {
	return genkit.DefineTool(
		g,
		"get_estimation_guide",
		`Provides a dynamically generated guide for what information is needed to estimate costs for ANY Google Cloud service.
This tool analyzes SKUs from the Cloud Billing Catalog API to generate accurate, up-to-date estimation requirements.

IMPORTANT: Call this tool FIRST before attempting to estimate costs. This ensures you gather all necessary information from the user through conversation.

=== WORKFLOW FOR ARCHITECTURE DIAGRAMS ===
When the user provides an architecture diagram (image):
1. Analyze the diagram to identify ALL GCP services/products used
2. Call this tool for EACH identified service to get required parameters
3. Ask the user about shared parameters first (e.g., region) then service-specific details
4. Use list_services and list_skus to find correct SKU IDs for each service
5. Call estimate_cost for EACH service
6. Sum up all estimates and present a total cost breakdown

=== WORKFLOW FOR SINGLE SERVICE ===
1. Call this tool to get parameter requirements (dynamically generated from SKU data)
2. Ask the user for the required information through conversation
3. Use list_services and list_skus to find specific SKUs
4. Call estimate_cost with the gathered information

=== FEATURES ===
- Dynamically analyzes SKUs to determine required parameters
- Retrieves free tier information from GCP documentation
- Works for ALL Google Cloud services (1800+ services)
- Always returns up-to-date pricing factors based on actual SKU data`,
		func(ctx *ai.ToolContext, input GetEstimationGuideInput) (*GetEstimationGuideOutput, error) {
			log.Printf("Tool 'get_estimation_guide' called for service: %s", input.ServiceName)

			if input.ServiceName == "" {
				return nil, fmt.Errorf("service_name is required")
			}

			// Find the service ID
			serviceID, displayName, err := findServiceByName(ctx.Context, pricingClient, input.ServiceName)
			if err != nil {
				log.Printf("Warning: Could not find service ID for %s: %v", input.ServiceName, err)
				// Continue without service ID - we can still provide a generic guide
			}

			var guide EstimationGuide
			guide.ServiceName = input.ServiceName
			if displayName != "" {
				guide.ServiceName = displayName
			}
			guide.ServiceID = serviceID

			// If we found a service ID, analyze its SKUs
			if serviceID != "" {
				skuGuide, err := analyzeSkusToGenerateGuide(ctx.Context, pricingClient, serviceID, guide.ServiceName)
				if err != nil {
					log.Printf("Warning: Could not analyze SKUs for %s: %v", input.ServiceName, err)
				} else {
					guide = *skuGuide
				}
			}

			// If we still don't have parameters, use generic template
			if len(guide.Parameters) == 0 {
				guide = buildGenericGuide(input.ServiceName)
			}

			// Fetch free tier information
			if freeTierService != nil {
				freeTierInfo, err := freeTierService.GetFreeTier(ctx.Context, input.ServiceName)
				if err == nil && freeTierInfo != nil {
					guide.FreeTier = &FreeTierSummary{
						Available: true,
						Items:     freeTierInfo.Items,
						Scope:     freeTierInfo.Scope,
						Period:    freeTierInfo.Period,
						SourceURL: freeTierInfo.SourceURL,
					}
				} else {
					guide.FreeTier = &FreeTierSummary{
						Available: false,
					}
				}
			}

			// Build suggested question
			suggestedQuestion := buildSuggestedQuestion(&guide)

			return &GetEstimationGuideOutput{
				Guide:             guide,
				SuggestedQuestion: suggestedQuestion,
			}, nil
		})
}

// findServiceByName searches for a GCP service by name and returns its ID
func findServiceByName(ctx context.Context, client *pricing.Client, serviceName string) (string, string, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(serviceName))

	// Fetch services from the API
	resp, err := client.ListServices(ctx, 5000, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to list services: %w", err)
	}

	// Try exact match first
	for _, svc := range resp.Services {
		if strings.ToLower(svc.DisplayName) == normalizedName {
			return svc.ServiceID, svc.DisplayName, nil
		}
	}

	// Try partial match
	for _, svc := range resp.Services {
		svcNameLower := strings.ToLower(svc.DisplayName)
		if strings.Contains(svcNameLower, normalizedName) || strings.Contains(normalizedName, svcNameLower) {
			return svc.ServiceID, svc.DisplayName, nil
		}
	}

	// Try matching common aliases
	aliases := getServiceAliases()
	if canonical, ok := aliases[normalizedName]; ok {
		for _, svc := range resp.Services {
			if strings.ToLower(svc.DisplayName) == canonical {
				return svc.ServiceID, svc.DisplayName, nil
			}
		}
	}

	return "", "", fmt.Errorf("service not found: %s", serviceName)
}

// getServiceAliases returns a map of common service name aliases to canonical names
func getServiceAliases() map[string]string {
	return map[string]string{
		"gke":                 "kubernetes engine",
		"k8s":                 "kubernetes engine",
		"gcs":                 "cloud storage",
		"bq":                  "bigquery",
		"gcf":                 "cloud functions",
		"gae":                 "app engine",
		"gce":                 "compute engine",
		"cloud run functions": "cloud functions",
		"2nd gen functions":   "cloud functions",
		"pubsub":              "pub/sub",
		"cloud pubsub":        "pub/sub",
	}
}

// analyzeSkusToGenerateGuide analyzes SKUs for a service and generates an estimation guide
func analyzeSkusToGenerateGuide(ctx context.Context, client *pricing.Client, serviceID, serviceName string) (*EstimationGuide, error) {
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

	// Analyze SKU descriptions to determine what parameters are needed
	descText := strings.ToLower(strings.Join(skuDescriptions, " "))
	catText := strings.ToLower(strings.Join(categories, " "))

	// Check for compute-related SKUs
	if containsAny(descText, []string{"vcpu", "cpu", "core", "instance"}) {
		params = append(params, RequiredParameter{
			Name:        "vcpu_count",
			Description: "Number of vCPUs",
			Required:    true,
			Examples:    []string{"1", "2", "4", "8"},
			DefaultTip:  "More vCPUs = higher cost but better performance",
		})
	}

	// Check for memory-related SKUs
	if containsAny(descText, []string{"memory", "ram", "gib"}) {
		params = append(params, RequiredParameter{
			Name:        "memory_gib",
			Description: "Memory in GiB",
			Required:    true,
			Examples:    []string{"1", "2", "4", "8", "16"},
			DefaultTip:  "Memory is typically charged per GiB-hour or GiB-second",
		})
	}

	// Check for storage-related SKUs
	if containsAny(descText, []string{"storage", "disk", "ssd", "hdd", "persistent"}) || containsAny(catText, []string{"storage"}) {
		params = append(params, RequiredParameter{
			Name:        "storage_gb",
			Description: "Storage capacity in GB",
			Required:    true,
			Examples:    []string{"10", "100", "500", "1000"},
			DefaultTip:  "Storage is typically charged per GB-month",
		})
	}

	// Check for request-based SKUs
	if containsAny(descText, []string{"request", "invocation", "call", "api"}) {
		params = append(params, RequiredParameter{
			Name:        "requests_per_month",
			Description: "Expected number of requests per month",
			Required:    true,
			Examples:    []string{"10000", "100000", "1000000"},
			DefaultTip:  "Many services have free tiers for requests",
		})
	}

	// Check for time-based billing
	if containsAny(descText, []string{"hour", "second", "minute", "time"}) {
		params = append(params, RequiredParameter{
			Name:        "monthly_hours",
			Description: "Expected running hours per month",
			Required:    true,
			Examples:    []string{"730 (24/7)", "176 (business hours)", "100"},
			DefaultTip:  "730 hours = full month of continuous operation",
		})
	}

	// Check for network/egress SKUs
	if containsAny(descText, []string{"egress", "network", "bandwidth", "transfer"}) {
		params = append(params, RequiredParameter{
			Name:        "egress_gb",
			Description: "Expected outbound data transfer in GB per month",
			Required:    false,
			Examples:    []string{"10", "100", "1000"},
			DefaultTip:  "Ingress is typically free, egress is charged",
		})
	}

	// Check for instance count
	if containsAny(descText, []string{"instance", "node", "replica"}) {
		params = append(params, RequiredParameter{
			Name:        "instance_count",
			Description: "Number of instances or nodes",
			Required:    true,
			Examples:    []string{"1", "2", "3", "5"},
			DefaultTip:  "More instances = higher availability but higher cost",
		})
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
