package tools

import (
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
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
	// Free tier information (Issue #8)
	TotalUsage        float64 `json:"total_usage"`
	FreeTierApplied   float64 `json:"free_tier_applied"`
	BillableUsage     float64 `json:"billable_usage"`
	FreeTierNote      string  `json:"free_tier_note,omitempty"`
	FreeTierSourceURL string  `json:"free_tier_source_url,omitempty"`
}

// EstimateCostOutput is the output of the estimate_cost tool
type EstimateCostOutput struct {
	Estimate CostBreakdown `json:"estimate"`
}

// NewEstimateCost creates a tool that estimates the cost based on usage
func NewEstimateCost(g *genkit.Genkit, client *pricing.Client, freeTierService *freetier.Service) ai.Tool {
	return genkit.DefineTool(
		g,
		"estimate_cost",
		`Estimates the cost for a specific SKU based on usage amount. 
Automatically applies free tier deductions when available and takes into account tiered pricing.

=== FREE TIER HANDLING ===
This tool automatically:
- Retrieves free tier information from GCP documentation
- Deducts free tier allowance from usage before calculating cost
- Reports free tier applied, billable usage, and source URL in the output

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

   | Service       | Description              | Free Tier | Monthly Cost |
   |---------------|--------------------------|-----------|--------------|
   | Cloud Run     | 2x instances, 1vCPU, 2GB | Applied   | $152.42      |
   | Cloud SQL     | PostgreSQL, db-custom-2  | N/A       | $89.50       |
   | Cloud Storage | 100GB Standard           | Applied   | $0.00        |
   | **Total**     |                          |           | **$241.92**  |

=== IMPORTANT ===
- DO NOT call this tool until you have gathered sufficient information from the user
- ALWAYS include service_name, region, and description parameters to track what each estimate covers
- For multi-service estimates, track each call and calculate the total at the end
- Note: Free tiers are typically per billing account, not per project`,
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

			// Track original usage for reporting
			totalUsage := input.UsageAmount
			billableUsage := input.UsageAmount
			var freeTierApplied float64
			var freeTierNote string
			var freeTierSourceURL string

			// Try to get and apply free tier information (Issue #8)
			if freeTierService != nil && input.ServiceName != "" {
				freeTierInfo, err := freeTierService.GetFreeTier(ctx.Context, input.ServiceName)
				if err == nil && freeTierInfo != nil {
					// Find matching free tier item for this SKU's usage unit
					matchingItem := freetier.FindMatchingFreeTierItem(freeTierInfo, rate.UnitInfo.Unit)
					if matchingItem != nil {
						// Calculate free tier deduction
						freeTierApplied = min(totalUsage, matchingItem.Amount)
						billableUsage = max(0, totalUsage-matchingItem.Amount)

						freeTierNote = fmt.Sprintf(
							"Free tier applied: %.0f %s (%s, %s)",
							matchingItem.Amount,
							matchingItem.Resource,
							freeTierInfo.Scope,
							freeTierInfo.Period,
						)
						freeTierSourceURL = freeTierInfo.SourceURL

						log.Printf("Free tier applied for %s: %.0f %s deducted, billable: %.0f",
							input.ServiceName, freeTierApplied, matchingItem.Resource, billableUsage)
					}
				}
			}

			// Calculate the cost based on billable usage (after free tier deduction)
			estimatedCost, err := client.CalculateCost(rate, billableUsage)
			if err != nil {
				log.Printf("Error calculating cost: %v", err)
				return nil, fmt.Errorf("failed to calculate cost: %w", err)
			}

			// Prepare output
			estimate := CostBreakdown{
				SKUID:         input.SKUID,
				UsageAmount:   totalUsage,
				EstimatedCost: estimatedCost,
				CurrencyCode:  currencyCode,
				Unit:          rate.UnitInfo.Unit,
				NumberOfTiers: len(rate.Tiers),
				TieredPricing: len(rate.Tiers) > 1,
				// Include context from input
				ServiceName: input.ServiceName,
				Region:      input.Region,
				Description: input.Description,
				// Free tier information
				TotalUsage:        totalUsage,
				FreeTierApplied:   freeTierApplied,
				BillableUsage:     billableUsage,
				FreeTierNote:      freeTierNote,
				FreeTierSourceURL: freeTierSourceURL,
			}

			// Calculate average price per unit for display (based on billable usage)
			if billableUsage > 0 {
				estimate.PricePerUnit = estimatedCost / billableUsage
			} else if len(rate.Tiers) > 0 {
				// Use first tier price if no billable usage
				tier := rate.Tiers[0]
				estimate.PricePerUnit = float64(tier.ListPrice.Nanos) / 1e9
			}

			// Create cost breakdown description
			var breakdownDesc string
			if freeTierApplied > 0 {
				breakdownDesc = fmt.Sprintf(
					"Total usage: %.2f %s. Free tier deducted: %.2f %s. Billable usage: %.2f %s. ",
					totalUsage, estimate.Unit,
					freeTierApplied, estimate.Unit,
					billableUsage, estimate.Unit,
				)
			}

			if estimate.TieredPricing {
				breakdownDesc += fmt.Sprintf(
					"Calculated using %d pricing tiers. Estimated cost: %.6f %s",
					estimate.NumberOfTiers,
					estimatedCost,
					currencyCode,
				)
			} else {
				breakdownDesc += fmt.Sprintf(
					"Flat rate: %.6f %s per unit = %.6f %s",
					estimate.PricePerUnit,
					currencyCode,
					estimatedCost,
					currencyCode,
				)
			}
			estimate.CostBreakdown = breakdownDesc

			return &EstimateCostOutput{
				Estimate: estimate,
			}, nil
		})
}
