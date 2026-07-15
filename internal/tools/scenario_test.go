package tools

import (
	"context"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/sebdah/goldie/v2"
)

// scenarioSnapshot pins a scripted multi-tool workflow and its final amounts.
// These are CI-safe automated regression tests: they do not call an LLM, but
// they lock the intended tool chain + cost outputs for representative prompts.
// Update fixtures with: go test ./internal/tools -run TestScenario -update
type scenarioSnapshot struct {
	Prompt             string               `json:"prompt"`
	Steps              []string             `json:"steps"`
	Estimates          []EstimateCostOutput `json:"estimates,omitempty"`
	Price              *GetSKUPriceOutput   `json:"price,omitempty"`
	TotalEstimatedCost float64              `json:"total_estimated_cost,omitempty"`
}

func cloudRunCPUSku() pricing.SKU {
	return pricing.SKU{
		SKUID:       "RUN-CPU-001",
		DisplayName: "CPU Allocation Time (vCPU-second)",
		ProductTaxonomy: pricing.ProductTaxonomy{TaxonomyCategories: []pricing.TaxonomyCategory{
			{Category: "Compute"},
		}},
		GeoTaxonomy: pricing.GeoTaxonomy{
			RegionalMetadata: pricing.RegionalMetadata{Region: pricing.Region{Region: "asia-northeast1"}},
		},
	}
}

func cloudStorageSku() pricing.SKU {
	return pricing.SKU{
		SKUID:       "GCS-STD-001",
		DisplayName: "Standard Storage us-multi-region",
		ProductTaxonomy: pricing.ProductTaxonomy{TaxonomyCategories: []pricing.TaxonomyCategory{
			{Category: "Storage"},
		}},
		GeoTaxonomy: pricing.GeoTaxonomy{Type: "GLOBAL"},
	}
}

func computeN1Sku() pricing.SKU {
	return pricing.SKU{
		SKUID:       "GCE-N1-001",
		DisplayName: "N1 Predefined Instance Core running in Tokyo",
		ProductTaxonomy: pricing.ProductTaxonomy{TaxonomyCategories: []pricing.TaxonomyCategory{
			{Category: "Compute"},
		}},
		GeoTaxonomy: pricing.GeoTaxonomy{
			RegionalMetadata: pricing.RegionalMetadata{Region: pricing.Region{Region: "asia-northeast1"}},
		},
	}
}

func TestScenario_QuickEstimateCloudRun(t *testing.T) {
	// Prompt: Cloud Run を asia-northeast1 で、1 vCPU / 2GB / 730時間/月の料金を見積もって
	// Expected flow: get_estimation_guide → estimate_cost (free tier applied)
	const vCPUSecondsPerMonth = 1 * 730 * 3600 // 1 vCPU × 730h

	client := &fakePricingClient{
		allServices:  []pricing.Service{{ServiceID: "SVC-RUN", DisplayName: "Cloud Run"}},
		listSKUsResp: &pricing.ListSKUsResponse{SKUs: []pricing.SKU{cloudRunCPUSku()}},
		priceResp:    flatRatePriceResp("USD", "0", 100000000, "s"), // $0.10 / second (fixture rate)
	}
	freeTier := &fakeFreeTierProvider{info: &freetier.FreeTierInfo{
		ServiceName: "Cloud Run",
		Items:       []freetier.FreeTierItem{{Resource: "vCPU-seconds", Amount: 180000, Unit: "seconds"}},
		Scope:       "account",
		Period:      "month",
		SourceURL:   "https://cloud.google.com/run/pricing",
	}}

	ctx := context.Background()
	steps := make([]string, 0, 2)

	guide, err := runGetEstimationGuide(ctx, client, freeTier, GetEstimationGuideInput{ServiceName: "Cloud Run"})
	if err != nil {
		t.Fatalf("get_estimation_guide: %v", err)
	}
	steps = append(steps, "get_estimation_guide")
	if guide.Guide.ServiceID != "SVC-RUN" {
		t.Fatalf("guide.ServiceID = %q, want SVC-RUN", guide.Guide.ServiceID)
	}

	estimate, err := runEstimateCost(ctx, client, freeTier, EstimateCostInput{
		SKUID:       cloudRunCPUSku().SKUID,
		UsageAmount: vCPUSecondsPerMonth,
		ServiceName: "Cloud Run",
		Region:      "asia-northeast1",
		Description: "1 vCPU Cloud Run, 730 hours/month",
	})
	if err != nil {
		t.Fatalf("estimate_cost: %v", err)
	}
	steps = append(steps, "estimate_cost")

	goldie.New(t).AssertJson(t, "scenario_quick_estimate_cloud_run", scenarioSnapshot{
		Prompt:    "Estimate monthly cost for Cloud Run in asia-northeast1: 1 vCPU, 2GB, 730 hours/month",
		Steps:     steps,
		Estimates: []EstimateCostOutput{*estimate},
	})
}

func TestScenario_MultiServiceRunAndStorage(t *testing.T) {
	// Prompt: Cloud Run + Cloud Storage 100GB の月額を合計で出して
	// Expected flow: get_estimation_guide×2 → estimate_cost×2 → sum
	client := &fakePricingClient{
		allServices: []pricing.Service{
			{ServiceID: "SVC-RUN", DisplayName: "Cloud Run"},
			{ServiceID: "SVC-GCS", DisplayName: "Cloud Storage"},
		},
		listSKUsByService: map[string]*pricing.ListSKUsResponse{
			"SVC-RUN": {SKUs: []pricing.SKU{cloudRunCPUSku()}},
			"SVC-GCS": {SKUs: []pricing.SKU{cloudStorageSku()}},
		},
		priceBySKU: map[string]*pricing.GetPriceResponse{
			"RUN-CPU-001": flatRatePriceResp("USD", "0", 100000000, "s"),
			"GCS-STD-001": flatRatePriceResp("USD", "0", 20000000, "GiBy"), // $0.02 / GiB
		},
	}
	freeTier := &fakeFreeTierProvider{
		infoByService: map[string]*freetier.FreeTierInfo{
			"Cloud Run": {
				ServiceName: "Cloud Run",
				Items:       []freetier.FreeTierItem{{Resource: "vCPU-seconds", Amount: 180000, Unit: "seconds"}},
				Scope:       "account",
				Period:      "month",
				SourceURL:   "https://cloud.google.com/run/pricing",
			},
			"Cloud Storage": {
				ServiceName: "Cloud Storage",
				Items:       []freetier.FreeTierItem{{Resource: "Standard Storage", Amount: 5, Unit: "GB-months"}},
				Scope:       "account",
				Period:      "month",
				SourceURL:   "https://cloud.google.com/storage/pricing",
			},
		},
	}

	ctx := context.Background()
	steps := make([]string, 0, 4)
	estimates := make([]EstimateCostOutput, 0, 2)

	for _, service := range []string{"Cloud Run", "Cloud Storage"} {
		if _, err := runGetEstimationGuide(ctx, client, freeTier, GetEstimationGuideInput{ServiceName: service}); err != nil {
			t.Fatalf("get_estimation_guide(%s): %v", service, err)
		}
		steps = append(steps, "get_estimation_guide:"+service)
	}

	runEst, err := runEstimateCost(ctx, client, freeTier, EstimateCostInput{
		SKUID:       "RUN-CPU-001",
		UsageAmount: 200000,
		ServiceName: "Cloud Run",
		Region:      "asia-northeast1",
		Description: "Cloud Run CPU seconds",
	})
	if err != nil {
		t.Fatalf("estimate_cost(Cloud Run): %v", err)
	}
	steps = append(steps, "estimate_cost:Cloud Run")
	estimates = append(estimates, *runEst)

	gcsEst, err := runEstimateCost(ctx, client, freeTier, EstimateCostInput{
		SKUID:       "GCS-STD-001",
		UsageAmount: 100,
		ServiceName: "Cloud Storage",
		Description: "100GB Standard Storage",
	})
	if err != nil {
		t.Fatalf("estimate_cost(Cloud Storage): %v", err)
	}
	steps = append(steps, "estimate_cost:Cloud Storage")
	estimates = append(estimates, *gcsEst)

	total := runEst.Estimate.EstimatedCost + gcsEst.Estimate.EstimatedCost
	goldie.New(t).AssertJson(t, "scenario_multi_service_run_and_storage", scenarioSnapshot{
		Prompt:             "Estimate monthly total for Cloud Run plus Cloud Storage 100GB",
		Steps:              steps,
		Estimates:          estimates,
		TotalEstimatedCost: total,
	})
}

func TestScenario_ExplorePricingCompute(t *testing.T) {
	// Prompt: Compute Engine の N1 系 SKU を tokyo で探して、単価だけ見せて
	// Expected flow: list_services → list_skus → get_sku_price (no estimate_cost)
	client := &fakePricingClient{
		allServices: []pricing.Service{
			{ServiceID: "SVC-GCE", DisplayName: "Compute Engine"},
			{ServiceID: "SVC-RUN", DisplayName: "Cloud Run"},
		},
		allSKUs: []pricing.SKU{computeN1Sku()},
		priceBySKU: map[string]*pricing.GetPriceResponse{
			"GCE-N1-001": flatRatePriceResp("USD", "0", 31600000, "h"), // $0.0316 / hour
		},
	}

	ctx := context.Background()
	steps := make([]string, 0, 3)

	services, err := runListServices(ctx, client, ListServicesInput{Name: "Compute Engine", CoreOnly: true})
	if err != nil {
		t.Fatalf("list_services: %v", err)
	}
	steps = append(steps, "list_services")
	if services.TotalReturned != 1 || services.Services[0].ServiceID != "SVC-GCE" {
		t.Fatalf("unexpected services: %+v", services.Services)
	}

	skus, err := runListSKUs(ctx, client, ListSKUsInput{
		ServiceID: services.Services[0].ServiceID,
		Region:    "asia-northeast1",
		Keyword:   "N1",
	})
	if err != nil {
		t.Fatalf("list_skus: %v", err)
	}
	steps = append(steps, "list_skus")
	if skus.TotalReturned != 1 || skus.SKUs[0].SKUID != "GCE-N1-001" {
		t.Fatalf("unexpected skus: %+v", skus.SKUs)
	}

	price, err := runGetSKUPrice(ctx, client, GetSKUPriceInput{SKUID: skus.SKUs[0].SKUID})
	if err != nil {
		t.Fatalf("get_sku_price: %v", err)
	}
	steps = append(steps, "get_sku_price")

	goldie.New(t).AssertJson(t, "scenario_explore_pricing_compute", scenarioSnapshot{
		Prompt: "Find Compute Engine N1 SKUs in Tokyo and show unit price only",
		Steps:  steps,
		Price:  price,
	})
}
