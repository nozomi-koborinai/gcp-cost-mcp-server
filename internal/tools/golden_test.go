package tools

import (
	"context"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/sebdah/goldie/v2"
)

// Characterization (golden) tests pin the full JSON output of cost-sensitive
// tools. Update fixtures with: go test ./internal/tools -update
func TestGolden_EstimateCost(t *testing.T) {
	g := goldie.New(t)

	tests := []struct {
		name     string
		client   *fakePricingClient
		freeTier FreeTierProvider
		input    EstimateCostInput
	}{
		{
			name:   "flat_rate",
			client: &fakePricingClient{priceResp: flatRatePriceResp("USD", "0", 100000000, "h")},
			input: EstimateCostInput{
				SKUID:       "TEST-FLAT",
				UsageAmount: 100,
			},
		},
		{
			name: "tiered_pricing",
			client: &fakePricingClient{priceResp: &pricing.GetPriceResponse{
				CurrencyCode: "USD",
				SKUPrices: []pricing.SKUPrice{{
					Rate: &pricing.Rate{
						Tiers: []pricing.Tier{
							{StartAmount: pricing.Amount{Value: "0"}, ListPrice: pricing.Money{Units: "0", Nanos: 200000000}},
							{StartAmount: pricing.Amount{Value: "100"}, ListPrice: pricing.Money{Units: "0", Nanos: 100000000}},
						},
						UnitInfo: pricing.UnitInfo{Unit: "GiBy"},
					},
				}},
			}},
			input: EstimateCostInput{
				SKUID:       "TEST-TIERED",
				UsageAmount: 150,
			},
		},
		{
			name:   "free_tier_applied",
			client: &fakePricingClient{priceResp: flatRatePriceResp("USD", "0", 100000000, "s")},
			freeTier: &fakeFreeTierProvider{info: &freetier.FreeTierInfo{
				ServiceName: "Cloud Run",
				Items:       []freetier.FreeTierItem{{Resource: "vCPU-seconds", Amount: 180000, Unit: "seconds"}},
				Scope:       "account",
				Period:      "month",
				SourceURL:   "https://cloud.google.com/run/pricing",
			}},
			input: EstimateCostInput{
				SKUID:       "TEST-FREE",
				UsageAmount: 200000,
				ServiceName: "Cloud Run",
				Region:      "asia-northeast1",
				Description: "1 vCPU Cloud Run, monthly",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runEstimateCost(context.Background(), tt.client, tt.freeTier, tt.input)
			if err != nil {
				t.Fatalf("runEstimateCost returned error: %v", err)
			}
			g.AssertJson(t, "estimate_cost_"+tt.name, out)
		})
	}
}

func TestGolden_GetSKUPrice(t *testing.T) {
	g := goldie.New(t)

	tests := []struct {
		name   string
		client *fakePricingClient
		input  GetSKUPriceInput
	}{
		{
			name:   "flat_rate",
			client: &fakePricingClient{priceResp: flatRatePriceResp("USD", "1", 500000000, "h")},
			input:  GetSKUPriceInput{SKUID: "TEST"},
		},
		{
			name: "tiered_and_committed",
			client: &fakePricingClient{priceResp: &pricing.GetPriceResponse{
				CurrencyCode: "USD",
				SKUPrices: []pricing.SKUPrice{
					{
						ConsumptionModel:            "DEFAULT",
						ConsumptionModelDescription: "On-demand pricing",
						ValueType:                   "rate",
						Rate: &pricing.Rate{
							Tiers: []pricing.Tier{
								{StartAmount: pricing.Amount{Value: "0"}, ListPrice: pricing.Money{CurrencyCode: "USD", Units: "0", Nanos: 24000}},
							},
							UnitInfo: pricing.UnitInfo{Unit: "s", UnitDescription: "second"},
						},
					},
					{
						ConsumptionModel:            "COMMITTED",
						ConsumptionModelDescription: "1 year commitment",
						ValueType:                   "rate",
						Rate: &pricing.Rate{
							Tiers: []pricing.Tier{
								{StartAmount: pricing.Amount{Value: "0"}, ListPrice: pricing.Money{CurrencyCode: "USD", Units: "0", Nanos: 17000}},
							},
							UnitInfo: pricing.UnitInfo{Unit: "s", UnitDescription: "second"},
						},
					},
				},
			}},
			input: GetSKUPriceInput{SKUID: "490F-75A9-E3F1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runGetSKUPrice(context.Background(), tt.client, tt.input)
			if err != nil {
				t.Fatalf("runGetSKUPrice returned error: %v", err)
			}
			g.AssertJson(t, "get_sku_price_"+tt.name, out)
		})
	}
}

func TestGolden_ClassifyResourceCost(t *testing.T) {
	g := goldie.New(t)
	client := WithSupplemental(&fakePricingClient{})

	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration.office",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(10),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost returned error: %v", err)
	}

	g.AssertJson(t, "classify_resource_cost_office_spla", out)
}
