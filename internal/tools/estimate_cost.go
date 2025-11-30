package tools

import (
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// EstimateCostInput is the input for the estimate_cost tool
type EstimateCostInput struct {
	SKUID        string  `json:"sku_id" jsonschema_description:"The SKU ID to calculate cost for (e.g., '0008-F633-76AA'). REQUIRED: Use list_skus to find SKU IDs first."`
	UsageAmount  float64 `json:"usage_amount" jsonschema_description:"The amount of usage to calculate cost for (in the SKU's unit, e.g., hours, GB, requests). REQUIRED."`
	CurrencyCode string  `json:"currency_code,omitempty" jsonschema_description:"ISO-4217 currency code (e.g., 'USD', 'JPY', 'EUR'). Defaults to USD if not specified."`
	// Additional context fields for better tracking
	ServiceName string `json:"service_name,omitempty" jsonschema_description:"The Google Cloud service name (e.g., 'Cloud Run', 'Compute Engine'). Helps track what this estimate is for."`
	Region      string `json:"region,omitempty" jsonschema_description:"The region for this estimate (e.g., 'asia-northeast1'). Important for accurate pricing."`
	Description string `json:"description,omitempty" jsonschema_description:"Description of what this estimate covers (e.g., '2 vCPU Cloud Run instance, 730 hours/month')."`
}

// CostBreakdown represents the cost calculation breakdown
type CostBreakdown struct {
	SKUID         string  `json:"sku_id"`
	UsageAmount   float64 `json:"usage_amount"`
	Unit          string  `json:"unit"`
	EstimatedCost float64 `json:"estimated_cost"`
	CurrencyCode  string  `json:"currency_code"`
	PricePerUnit  float64 `json:"price_per_unit"`
	TieredPricing bool    `json:"tiered_pricing"`
	NumberOfTiers int     `json:"number_of_tiers"`
	CostBreakdown string  `json:"cost_breakdown,omitempty"`
	// Additional context in output
	ServiceName string `json:"service_name,omitempty"`
	Region      string `json:"region,omitempty"`
	Description string `json:"description,omitempty"`
}

// EstimateCostOutput is the output of the estimate_cost tool
type EstimateCostOutput struct {
	Estimate CostBreakdown `json:"estimate"`
}

// NewEstimateCost creates a tool that estimates the cost based on usage
func NewEstimateCost(g *genkit.Genkit, client *pricing.Client) ai.Tool {
	return genkit.DefineTool(
		g,
		"estimate_cost",
		`Estimates the cost for a specific SKU based on usage amount. Takes into account tiered pricing if applicable.

=== SINGLE SERVICE WORKFLOW ===
1. FIRST call get_estimation_guide to understand what information is needed
2. ASK the user for required details (region, specs, usage patterns, etc.)
3. Use list_services to find the service ID
4. Use list_skus to find the correct SKU for the requirements
5. Call this tool with the SKU ID and usage amount

=== MULTI-SERVICE / ARCHITECTURE DIAGRAM WORKFLOW ===
When estimating costs for multiple services (e.g., from an architecture diagram):
1. Call get_estimation_guide for EACH service identified
2. Gather parameters from user (shared params like region first, then service-specific)
3. For EACH service: list_services → list_skus → estimate_cost
4. Call this tool multiple times (once per service/SKU)
5. SUM all estimates and present a breakdown table like:

   | Service       | Description              | Monthly Cost |
   |---------------|--------------------------|--------------|
   | Cloud Run     | 2x instances, 1vCPU, 2GB | $152.42      |
   | Cloud SQL     | PostgreSQL, db-custom-2  | $89.50       |
   | Cloud Storage | 100GB Standard           | $2.30        |
   | **Total**     |                          | **$244.22**  |

=== IMPORTANT ===
- DO NOT call this tool until you have gathered sufficient information from the user
- ALWAYS include service_name, region, and description parameters to track what each estimate covers
- For multi-service estimates, track each call and calculate the total at the end`,
		func(ctx *ai.ToolContext, input EstimateCostInput) (*EstimateCostOutput, error) {
			log.Printf("Tool 'estimate_cost' called for sku_id: %s, usage: %f", input.SKUID, input.UsageAmount)

			if input.SKUID == "" {
				return nil, fmt.Errorf("sku_id is required")
			}

			if input.UsageAmount < 0 {
				return nil, fmt.Errorf("usage_amount must be non-negative")
			}

			// Default to USD if not specified
			currencyCode := input.CurrencyCode
			if currencyCode == "" {
				currencyCode = "USD"
			}

			// Get the price for this SKU
			priceResp, err := client.GetSKUPrice(ctx.Context, input.SKUID, currencyCode)
			if err != nil {
				log.Printf("Error getting SKU price: %v", err)
				return nil, fmt.Errorf("failed to get SKU price: %w", err)
			}

			// Extract rate from the first SKUPrice entry
			if len(priceResp.SKUPrices) == 0 || priceResp.SKUPrices[0].Rate == nil {
				return nil, fmt.Errorf("no pricing data available for SKU %s", input.SKUID)
			}
			rate := priceResp.SKUPrices[0].Rate

			// Calculate the cost
			estimatedCost, err := client.CalculateCost(rate, input.UsageAmount)
			if err != nil {
				log.Printf("Error calculating cost: %v", err)
				return nil, fmt.Errorf("failed to calculate cost: %w", err)
			}

			// Prepare output
			estimate := CostBreakdown{
				SKUID:         input.SKUID,
				UsageAmount:   input.UsageAmount,
				EstimatedCost: estimatedCost,
				CurrencyCode:  currencyCode,
				Unit:          rate.UnitInfo.Unit,
				NumberOfTiers: len(rate.Tiers),
				TieredPricing: len(rate.Tiers) > 1,
				// Include context from input
				ServiceName: input.ServiceName,
				Region:      input.Region,
				Description: input.Description,
			}

			// Calculate average price per unit for display
			if input.UsageAmount > 0 {
				estimate.PricePerUnit = estimatedCost / input.UsageAmount
			} else if len(rate.Tiers) > 0 {
				// Use first tier price if no usage
				tier := rate.Tiers[0]
				estimate.PricePerUnit = float64(tier.ListPrice.Nanos) / 1e9
			}

			// Create cost breakdown description
			if estimate.TieredPricing {
				estimate.CostBreakdown = fmt.Sprintf(
					"Calculated using %d pricing tiers. Usage of %.2f %s results in estimated cost of %.6f %s",
					estimate.NumberOfTiers,
					input.UsageAmount,
					estimate.Unit,
					estimatedCost,
					currencyCode,
				)
			} else {
				estimate.CostBreakdown = fmt.Sprintf(
					"Flat rate pricing. Usage of %.2f %s at %.6f %s per unit = %.6f %s",
					input.UsageAmount,
					estimate.Unit,
					estimate.PricePerUnit,
					currencyCode,
					estimatedCost,
					currencyCode,
				)
			}

			return &EstimateCostOutput{
				Estimate: estimate,
			}, nil
		})
}
