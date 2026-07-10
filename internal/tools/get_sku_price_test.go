package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

func flatRatePriceResp(currency string, units string, nanos int64, unit string) *pricing.GetPriceResponse {
	return &pricing.GetPriceResponse{
		Name:         "skus/TEST/price",
		CurrencyCode: currency,
		SKUPrices: []pricing.SKUPrice{{
			ConsumptionModel: "CONSUMPTION_MODEL_ON_DEMAND",
			ValueType:        "rate",
			Rate: &pricing.Rate{
				Tiers: []pricing.Tier{{
					StartAmount: pricing.Amount{Value: "0"},
					ListPrice:   pricing.Money{CurrencyCode: currency, Units: units, Nanos: nanos},
				}},
				UnitInfo: pricing.UnitInfo{Unit: unit, UnitDescription: "hour"},
			},
		}},
	}
}

func TestRunGetSKUPrice_FlatRate(t *testing.T) {
	client := &fakePricingClient{priceResp: flatRatePriceResp("USD", "1", 500000000, "h")}

	out, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{SKUID: "TEST"})
	if err != nil {
		t.Fatalf("runGetSKUPrice returned error: %v", err)
	}
	if out.Price.Unit != "h" {
		t.Errorf("Unit = %q, want h", out.Price.Unit)
	}
	if len(out.Price.Tiers) != 1 {
		t.Fatalf("len(Tiers) = %d, want 1", len(out.Price.Tiers))
	}
	if got, want := out.Price.Tiers[0].PricePerUnit, 1.5; got != want {
		t.Errorf("PricePerUnit = %v, want %v", got, want)
	}
	if len(out.Price.AllPricingModels) != 1 {
		t.Errorf("len(AllPricingModels) = %d, want 1", len(out.Price.AllPricingModels))
	}
}

func TestRunGetSKUPrice_EmptySKUID(t *testing.T) {
	client := &fakePricingClient{}

	if _, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{}); err == nil {
		t.Fatal("runGetSKUPrice should return error when sku_id is empty")
	}
}

func TestRunGetSKUPrice_APIError(t *testing.T) {
	client := &fakePricingClient{priceErr: errors.New("api down")}

	if _, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{SKUID: "TEST"}); err == nil {
		t.Fatal("runGetSKUPrice should return error when API fails")
	}
}

func TestRunGetSKUPrice_InvalidPriceData(t *testing.T) {
	resp := flatRatePriceResp("USD", "not-a-number", 0, "h")
	client := &fakePricingClient{priceResp: resp}

	if _, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{SKUID: "TEST"}); err == nil {
		t.Fatal("runGetSKUPrice should return error for invalid price units")
	}
}
