package tools

import (
	"context"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
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

// ConsumptionPricing represents pricing for a specific consumption model (e.g., on-demand, CUD)
type ConsumptionPricing struct {
	ConsumptionModel string        `json:"consumption_model"`
	Description      string        `json:"description,omitempty"`
	Unit             string        `json:"unit"`
	UnitDescription  string        `json:"unit_description"`
	Tiers            []PricingTier `json:"tiers"`
	AggregationInfo  string        `json:"aggregation_info,omitempty"`
}

// PriceInfo represents pricing information
type PriceInfo struct {
	SKUID              string               `json:"sku_id"`
	CurrencyCode       string               `json:"currency_code"`
	Unit               string               `json:"unit"`
	UnitDescription    string               `json:"unit_description"`
	Tiers              []PricingTier         `json:"tiers"`
	AggregationInfo    string               `json:"aggregation_info,omitempty"`
	AllPricingModels   []ConsumptionPricing  `json:"all_pricing_models,omitempty"`
}

// GetSKUPriceOutput is the output of the get_sku_price tool
type GetSKUPriceOutput struct {
	Price PriceInfo `json:"price"`
}

const getSKUPriceDescription = "Gets detailed pricing information for a specific SKU. Returns the price per unit and any tiered pricing information. IMPORTANT: You must first use list_skus to obtain the SKU ID before calling this tool."

// NewGetSKUPrice creates a tool that gets the price for a specific SKU
func NewGetSKUPrice(g *genkit.Genkit, client PricingClient) ai.Tool {
	return genkit.DefineTool(
		g,
		"get_sku_price",
		getSKUPriceDescription,
		func(ctx *ai.ToolContext, input GetSKUPriceInput) (*GetSKUPriceOutput, error) {
			return runGetSKUPrice(ctx.Context, client, input)
		})
}

func runGetSKUPrice(ctx context.Context, client PricingClient, input GetSKUPriceInput) (*GetSKUPriceOutput, error) {
	log.Printf("Tool 'get_sku_price' called for sku_id: %s, currency: %s", input.SKUID, input.CurrencyCode)

	if input.SKUID == "" {
		return nil, fmt.Errorf("sku_id is required")
	}

	currencyCode := input.CurrencyCode
	if currencyCode == "" {
		currencyCode = "USD"
	}

	resp, err := client.GetSKUPrice(ctx, input.SKUID, currencyCode)
	if err != nil {
		log.Printf("Error getting SKU price: %v", err)
		return nil, fmt.Errorf("failed to get SKU price: %w", err)
	}

	priceInfo := PriceInfo{
		SKUID:        input.SKUID,
		CurrencyCode: resp.CurrencyCode,
		Tiers:        []PricingTier{},
	}

	var allModels []ConsumptionPricing

	for idx, skuPrice := range resp.SKUPrices {
		if skuPrice.Rate == nil {
			continue
		}
		rate := skuPrice.Rate

		model := ConsumptionPricing{
			ConsumptionModel: skuPrice.ConsumptionModel,
			Description:      skuPrice.ConsumptionModelDescription,
			Unit:             rate.UnitInfo.Unit,
			UnitDescription:  rate.UnitInfo.UnitDescription,
			Tiers:            []PricingTier{},
		}

		if rate.AggregationInfo.Level != "" || rate.AggregationInfo.Interval != "" {
			model.AggregationInfo = fmt.Sprintf("%s / %s",
				rate.AggregationInfo.Level,
				rate.AggregationInfo.Interval)
		}

		for _, tier := range rate.Tiers {
			startAmount, err := tier.StartAmount.Float64()
			if err != nil {
				return nil, fmt.Errorf("invalid pricing data for SKU %s: %w", input.SKUID, err)
			}
			pricePerUnit, err := tier.ListPrice.UnitPrice()
			if err != nil {
				return nil, fmt.Errorf("invalid pricing data for SKU %s: %w", input.SKUID, err)
			}

			model.Tiers = append(model.Tiers, PricingTier{
				StartAmount:  startAmount,
				PricePerUnit: pricePerUnit,
				Currency:     tier.ListPrice.CurrencyCode,
			})
		}

		allModels = append(allModels, model)

		if idx == 0 {
			priceInfo.Unit = rate.UnitInfo.Unit
			priceInfo.UnitDescription = rate.UnitInfo.UnitDescription
			priceInfo.AggregationInfo = model.AggregationInfo
			priceInfo.Tiers = model.Tiers
		}
	}

	if len(allModels) > 0 {
		priceInfo.AllPricingModels = allModels
	}

	return &GetSKUPriceOutput{
		Price: priceInfo,
	}, nil
}
