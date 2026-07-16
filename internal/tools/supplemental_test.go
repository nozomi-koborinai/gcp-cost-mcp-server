package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing/supplemental"
)

func TestWithSupplemental_ListServicesFindsOffice(t *testing.T) {
	inner := &fakePricingClient{
		allServices: []pricing.Service{
			{ServiceID: "S1", DisplayName: "Cloud Run"},
			{ServiceID: "S2", DisplayName: "Compute Engine"},
		},
	}
	client := WithSupplemental(inner)

	out, err := runListServices(context.Background(), client, ListServicesInput{Name: "Office"})
	if err != nil {
		t.Fatalf("runListServices: %v", err)
	}
	if out.TotalReturned != 1 {
		t.Fatalf("TotalReturned = %d, want 1; services=%v", out.TotalReturned, out.Services)
	}
	if out.Services[0].ServiceID != supplemental.ServiceIDLicenseManager {
		t.Fatalf("ServiceID = %q, want %q", out.Services[0].ServiceID, supplemental.ServiceIDLicenseManager)
	}
}

func TestWithSupplemental_ListServicesAppendsOnLastPage(t *testing.T) {
	inner := &fakePricingClient{
		listServicesResp: &pricing.ListServicesResponse{
			Services: []pricing.Service{{ServiceID: "S1", DisplayName: "Cloud Run"}},
		},
	}
	client := WithSupplemental(inner)

	out, err := runListServices(context.Background(), client, ListServicesInput{})
	if err != nil {
		t.Fatalf("runListServices: %v", err)
	}
	found := false
	for _, svc := range out.Services {
		if svc.ServiceID == supplemental.ServiceIDLicenseManager {
			found = true
		}
	}
	if !found {
		t.Fatalf("License Manager not appended; services=%v", out.Services)
	}
}

func TestWithSupplemental_ListSKUsAndEstimateOffice(t *testing.T) {
	inner := &fakePricingClient{
		// Ensure Catalog path is not used for supplemental IDs.
		listSKUsErr: errors.New("should not call catalog for supplemental service"),
		priceErr:    errors.New("should not call catalog for supplemental SKU"),
	}
	client := WithSupplemental(inner)

	skusOut, err := runListSKUs(context.Background(), client, ListSKUsInput{
		ServiceID: supplemental.ServiceIDLicenseManager,
		Keyword:   "Office",
	})
	if err != nil {
		t.Fatalf("runListSKUs: %v", err)
	}
	if skusOut.TotalReturned != 1 {
		t.Fatalf("TotalReturned = %d, want 1", skusOut.TotalReturned)
	}
	sku := skusOut.SKUs[0]
	if sku.SKUID != supplemental.SKUIDOfficeLTSC2021ProPlus {
		t.Fatalf("SKUID = %q, want %q", sku.SKUID, supplemental.SKUIDOfficeLTSC2021ProPlus)
	}
	if sku.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("BillingModel = %q, want %q", sku.BillingModel, supplemental.BillingModelExistence)
	}
	if len(sku.BillingNotes) == 0 || sku.SourceURL == "" {
		t.Fatalf("expected billing notes and source URL, got notes=%v url=%q", sku.BillingNotes, sku.SourceURL)
	}

	priceOut, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{
		SKUID: sku.SKUID,
	})
	if err != nil {
		t.Fatalf("runGetSKUPrice: %v", err)
	}
	if priceOut.Price.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("price BillingModel = %q", priceOut.Price.BillingModel)
	}
	if len(priceOut.Price.Tiers) != 1 || priceOut.Price.Tiers[0].PricePerUnit != 21.40 {
		t.Fatalf("unexpected tiers: %+v", priceOut.Price.Tiers)
	}

	estOut, err := runEstimateCost(context.Background(), client, nil, EstimateCostInput{
		SKUID:       sku.SKUID,
		UsageAmount: 10, // 10 authorized users
	})
	if err != nil {
		t.Fatalf("runEstimateCost: %v", err)
	}
	if estOut.Estimate.EstimatedCost != 214.0 {
		t.Fatalf("EstimatedCost = %v, want 214.0", estOut.Estimate.EstimatedCost)
	}
	if estOut.Estimate.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("estimate BillingModel = %q", estOut.Estimate.BillingModel)
	}
	if estOut.Estimate.ServiceName == "" {
		t.Fatal("expected ServiceName to be filled from supplemental catalog")
	}
}

func TestGetEstimationGuide_LicenseManager(t *testing.T) {
	// Inner client unused for supplemental guide path.
	client := WithSupplemental(&fakePricingClient{allServicesErr: errors.New("unused")})

	out, err := runGetEstimationGuide(context.Background(), client, nil, GetEstimationGuideInput{
		ServiceName: "Office SPLA",
	})
	if err != nil {
		t.Fatalf("runGetEstimationGuide: %v", err)
	}
	if out.Guide.ServiceID != supplemental.ServiceIDLicenseManager {
		t.Fatalf("ServiceID = %q, want %q", out.Guide.ServiceID, supplemental.ServiceIDLicenseManager)
	}
	if len(out.Guide.Parameters) == 0 {
		t.Fatal("expected guide parameters")
	}
	foundLicenseCount := false
	for _, p := range out.Guide.Parameters {
		if p.Name == "license_count" {
			foundLicenseCount = true
		}
	}
	if !foundLicenseCount {
		t.Fatalf("license_count parameter missing: %+v", out.Guide.Parameters)
	}
	joinedTips := strings.Join(out.Guide.Tips, " ")
	if !strings.Contains(strings.ToLower(joinedTips), "catalog") {
		t.Fatalf("expected tip mentioning Catalog absence, tips=%v", out.Guide.Tips)
	}
}

func TestWithSupplemental_PassthroughCatalogSKU(t *testing.T) {
	inner := &fakePricingClient{
		priceResp: flatRatePriceResp("USD", "1", 0, "h"),
	}
	client := WithSupplemental(inner)

	out, err := runGetSKUPrice(context.Background(), client, GetSKUPriceInput{SKUID: "0008-F633-76AA"})
	if err != nil {
		t.Fatalf("runGetSKUPrice: %v", err)
	}
	if out.Price.BillingModel != "" {
		t.Fatalf("catalog SKU should not get supplemental billing_model, got %q", out.Price.BillingModel)
	}
	if out.Price.Tiers[0].PricePerUnit != 1.0 {
		t.Fatalf("PricePerUnit = %v, want 1.0", out.Price.Tiers[0].PricePerUnit)
	}
}
