package tools

import (
	"fmt"
	"log"
	"strconv"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// GetSKUPriceInput is the input for the get_sku_price tool
type GetSKUPriceInput struct {
	SKUID        string `json:"sku_id" jsonschema_description:"The SKU ID to get pricing for (e.g., '0008-F633-76AA'). Use list_skus to find SKU IDs."`
	CurrencyCode string `json:"currency_code,omitempty" jsonschema_description:"ISO-4217 currency code (e.g., 'USD', 'JPY', 'EUR'). Defaults to USD if not specified."`
}

// PricingTier represents a pricing tier
type PricingTier struct {
	StartAmount  float64 `json:"start_amount"`
	PricePerUnit float64 `json:"price_per_unit"`
	Currency     string  `json:"currency"`
}

// PriceInfo represents pricing information
type PriceInfo struct {
	SKUID           string        `json:"sku_id"`
	CurrencyCode    string        `json:"currency_code"`
	Unit            string        `json:"unit"`
	UnitDescription string        `json:"unit_description"`
	Tiers           []PricingTier `json:"tiers"`
	AggregationInfo string        `json:"aggregation_info,omitempty"`
}

// GetSKUPriceOutput is the output of the get_sku_price tool
type GetSKUPriceOutput struct {
	Price PriceInfo `json:"price"`
}

// NewGetSKUPrice creates a tool that gets the price for a specific SKU
func NewGetSKUPrice(g *genkit.Genkit, client *pricing.Client) ai.Tool {
	return genkit.DefineTool(
		g,
		"get_sku_price",
		"Gets detailed pricing information for a specific SKU. Returns the price per unit and any tiered pricing information. IMPORTANT: You must first use list_skus to obtain the SKU ID before calling this tool.",
		func(ctx *ai.ToolContext, input GetSKUPriceInput) (*GetSKUPriceOutput, error) {
			log.Printf("Tool 'get_sku_price' called for sku_id: %s, currency: %s", input.SKUID, input.CurrencyCode)

			if input.SKUID == "" {
				return nil, fmt.Errorf("sku_id is required")
			}

			// Default to USD if not specified
			currencyCode := input.CurrencyCode
			if currencyCode == "" {
				currencyCode = "USD"
			}

			resp, err := client.GetSKUPrice(ctx.Context, input.SKUID, currencyCode)
			if err != nil {
				log.Printf("Error getting SKU price: %v", err)
				return nil, fmt.Errorf("failed to get SKU price: %w", err)
			}

			priceInfo := PriceInfo{
				SKUID:        input.SKUID,
				CurrencyCode: resp.CurrencyCode,
				Tiers:        []PricingTier{}, // Initialize as empty slice to avoid null in JSON
			}

			// Extract rate from the first SKUPrice entry (typically "Default" consumption model)
			if len(resp.SKUPrices) > 0 && resp.SKUPrices[0].Rate != nil {
				rate := resp.SKUPrices[0].Rate
				priceInfo.Unit = rate.UnitInfo.Unit
				priceInfo.UnitDescription = rate.UnitInfo.UnitDescription

				// Build aggregation info string
				if rate.AggregationInfo.Level != "" || rate.AggregationInfo.Interval != "" {
					priceInfo.AggregationInfo = fmt.Sprintf("%s / %s",
						rate.AggregationInfo.Level,
						rate.AggregationInfo.Interval)
				}

				// Convert tiers
				for _, tier := range rate.Tiers {
					startAmount, _ := strconv.ParseFloat(tier.StartAmount.Value, 64)
					units, _ := strconv.ParseFloat(tier.ListPrice.Units, 64)
					nanos := float64(tier.ListPrice.Nanos) / 1e9
					pricePerUnit := units + nanos

					priceInfo.Tiers = append(priceInfo.Tiers, PricingTier{
						StartAmount:  startAmount,
						PricePerUnit: pricePerUnit,
						Currency:     tier.ListPrice.CurrencyCode,
					})
				}
			}

			return &GetSKUPriceOutput{
				Price: priceInfo,
			}, nil
		})
}
