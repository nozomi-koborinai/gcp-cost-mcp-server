package tools

import (
	"context"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
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

const getEstimationGuideDescription = `Provides a dynamically generated guide for what information is needed to estimate costs for ANY Google Cloud service.
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
- Always returns up-to-date pricing factors based on actual SKU data`

// NewGetEstimationGuide creates a tool that provides estimation requirements for GCP services
func NewGetEstimationGuide(g *genkit.Genkit, pricingClient PricingClient, freeTierService FreeTierProvider) ai.Tool {
	return genkit.DefineTool(
		g,
		"get_estimation_guide",
		getEstimationGuideDescription,
		func(ctx *ai.ToolContext, input GetEstimationGuideInput) (*GetEstimationGuideOutput, error) {
			return runGetEstimationGuide(ctx.Context, pricingClient, freeTierService, input)
		})
}

func runGetEstimationGuide(ctx context.Context, pricingClient PricingClient, freeTierService FreeTierProvider, input GetEstimationGuideInput) (*GetEstimationGuideOutput, error) {
	log.Printf("Tool 'get_estimation_guide' called for service: %s", input.ServiceName)

	if input.ServiceName == "" {
		return nil, fmt.Errorf("service_name is required")
	}

	// Find the service ID
	serviceID, displayName, err := findServiceByName(ctx, pricingClient, input.ServiceName)
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
		skuGuide, err := analyzeSkusToGenerateGuide(ctx, pricingClient, serviceID, guide.ServiceName)
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
		freeTierInfo, err := freeTierService.GetFreeTier(ctx, input.ServiceName)
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
}
