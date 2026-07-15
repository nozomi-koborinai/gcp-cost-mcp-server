package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

func TestRunEstimateCost_FlatRate(t *testing.T) {
	client := &fakePricingClient{priceResp: flatRatePriceResp("USD", "0", 100000000, "h")}

	out, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{
		SKUID:       "TEST",
		UsageAmount: 100,
	})
	if err != nil {
		t.Fatalf("runEstimateCost returned error: %v", err)
	}
	if got, want := out.Estimate.EstimatedCost, 10.0; got != want {
		t.Errorf("EstimatedCost = %v, want %v", got, want)
	}
	if out.Estimate.CurrencyCode != "USD" {
		t.Errorf("CurrencyCode = %q, want USD (default)", out.Estimate.CurrencyCode)
	}
	if out.Estimate.TieredPricing {
		t.Error("TieredPricing = true, want false for single tier")
	}
	if out.Estimate.BillableUsage != 100 {
		t.Errorf("BillableUsage = %v, want 100 (no free tier)", out.Estimate.BillableUsage)
	}
}

func TestRunEstimateCost_TieredPricing(t *testing.T) {
	client := &fakePricingClient{priceResp: &pricing.GetPriceResponse{
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
	}}

	out, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{
		SKUID:       "TEST",
		UsageAmount: 150,
	})
	if err != nil {
		t.Fatalf("runEstimateCost returned error: %v", err)
	}
	// 100 * 0.2 + 50 * 0.1 = 25
	if got, want := out.Estimate.EstimatedCost, 25.0; got != want {
		t.Errorf("EstimatedCost = %v, want %v", got, want)
	}
	if !out.Estimate.TieredPricing || out.Estimate.NumberOfTiers != 2 {
		t.Errorf("TieredPricing/NumberOfTiers = %v/%d, want true/2", out.Estimate.TieredPricing, out.Estimate.NumberOfTiers)
	}
}

func TestRunEstimateCost_FreeTierApplied(t *testing.T) {
	client := &fakePricingClient{priceResp: flatRatePriceResp("USD", "0", 100000000, "s")}
	freeTier := &fakeFreeTierProvider{info: &freetier.FreeTierInfo{
		ServiceName: "Cloud Run",
		Items:       []freetier.FreeTierItem{{Resource: "vCPU-seconds", Amount: 180000, Unit: "seconds"}},
		Scope:       "account",
		Period:      "month",
		SourceURL:   "https://cloud.google.com/run/pricing",
	}}

	out, err := runEstimateCost(context.Background(), client, freeTier, EstimateCostInput{
		SKUID:       "TEST",
		UsageAmount: 200000,
		ServiceName: "Cloud Run",
	})
	if err != nil {
		t.Fatalf("runEstimateCost returned error: %v", err)
	}
	if got, want := out.Estimate.FreeTierApplied, 180000.0; got != want {
		t.Errorf("FreeTierApplied = %v, want %v", got, want)
	}
	if got, want := out.Estimate.BillableUsage, 20000.0; got != want {
		t.Errorf("BillableUsage = %v, want %v", got, want)
	}
	// 20000 * 0.1 = 2000
	if got, want := out.Estimate.EstimatedCost, 2000.0; got != want {
		t.Errorf("EstimatedCost = %v, want %v", got, want)
	}
	if out.Estimate.FreeTierSourceURL != "https://cloud.google.com/run/pricing" {
		t.Errorf("FreeTierSourceURL = %q", out.Estimate.FreeTierSourceURL)
	}
}

func TestRunEstimateCost_FreeTierUnavailable(t *testing.T) {
	client := &fakePricingClient{priceResp: flatRatePriceResp("USD", "0", 100000000, "h")}
	freeTier := &fakeFreeTierProvider{err: errors.New("not found")}

	out, err := runEstimateCost(context.Background(), client, freeTier, EstimateCostInput{
		SKUID:       "TEST",
		UsageAmount: 100,
		ServiceName: "Cloud Run",
	})
	if err != nil {
		t.Fatalf("runEstimateCost returned error: %v", err)
	}
	if out.Estimate.FreeTierApplied != 0 || out.Estimate.BillableUsage != 100 {
		t.Errorf("free tier should not be applied: %+v", out.Estimate)
	}
}

func TestRunEstimateCost_Validation(t *testing.T) {
	client := &fakePricingClient{}

	if _, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{UsageAmount: 1}); err == nil {
		t.Fatal("runEstimateCost should return error when sku_id is empty")
	}
	if _, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{SKUID: "TEST", UsageAmount: -1}); err == nil {
		t.Fatal("runEstimateCost should return error when usage_amount is negative")
	}
}

func TestRunEstimateCost_NoPricingData(t *testing.T) {
	client := &fakePricingClient{priceResp: &pricing.GetPriceResponse{CurrencyCode: "USD"}}

	if _, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{SKUID: "TEST", UsageAmount: 1}); err == nil {
		t.Fatal("runEstimateCost should return error when no pricing data is available")
	}
}
