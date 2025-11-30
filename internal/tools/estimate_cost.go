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
	SKUID        string  `json:"sku_id" jsonschema_description:"The SKU ID to calculate cost for (e.g., '0008-F633-76AA'). Use list_skus to find SKU IDs."`
	UsageAmount  float64 `json:"usage_amount" jsonschema_description:"The amount of usage to calculate cost for (in the SKU's unit, e.g., hours, GB, requests)"`
	CurrencyCode string  `json:"currency_code,omitempty" jsonschema_description:"ISO-4217 currency code (e.g., 'USD', 'JPY', 'EUR'). Defaults to USD if not specified."`
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
		"Estimates the cost for a specific SKU based on usage amount. Takes into account tiered pricing if applicable. IMPORTANT: You must first use list_skus to obtain the SKU ID before calling this tool.",
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
