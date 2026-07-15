package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

func TestRunGetEstimationGuide_ServiceFound(t *testing.T) {
	client := &fakePricingClient{
		allServices: []pricing.Service{{ServiceID: "ABCD-1234", DisplayName: "Cloud Run"}},
		listSKUsResp: &pricing.ListSKUsResponse{SKUs: []pricing.SKU{
			{
				SKUID:       "K1",
				DisplayName: "CPU Allocation Time (vCPU-second)",
				ProductTaxonomy: pricing.ProductTaxonomy{TaxonomyCategories: []pricing.TaxonomyCategory{
					{Category: "Compute"},
				}},
				GeoTaxonomy: pricing.GeoTaxonomy{
					RegionalMetadata: pricing.RegionalMetadata{Region: pricing.Region{Region: "asia-northeast1"}},
				},
			},
		}},
	}
	freeTier := &fakeFreeTierProvider{info: &freetier.FreeTierInfo{
		ServiceName: "Cloud Run",
		Items:       []freetier.FreeTierItem{{Resource: "vCPU-seconds", Amount: 180000, Unit: "seconds"}},
		Scope:       "account",
		Period:      "month",
		SourceURL:   "https://cloud.google.com/run/pricing",
	}}

	out, err := runGetEstimationGuide(context.Background(), client, freeTier, GetEstimationGuideInput{ServiceName: "cloud run"})
	if err != nil {
		t.Fatalf("runGetEstimationGuide returned error: %v", err)
	}
	if out.Guide.ServiceID != "ABCD-1234" {
		t.Errorf("ServiceID = %q, want ABCD-1234", out.Guide.ServiceID)
	}
	if out.Guide.ServiceName != "Cloud Run" {
		t.Errorf("ServiceName = %q, want Cloud Run (display name)", out.Guide.ServiceName)
	}
	if len(out.Guide.Parameters) == 0 || out.Guide.Parameters[0].Name != "region" {
		t.Fatalf("first parameter should be region: %+v", out.Guide.Parameters)
	}
	// "CPU Allocation Time (vCPU-second)" contains "cpu" and "second" keywords
	if !hasParameter(out.Guide.Parameters, "vcpu_count") {
		t.Errorf("expected vcpu_count parameter, got %+v", out.Guide.Parameters)
	}
	if out.Guide.FreeTier == nil || !out.Guide.FreeTier.Available {
		t.Errorf("FreeTier should be available: %+v", out.Guide.FreeTier)
	}
	if out.SuggestedQuestion == "" {
		t.Error("SuggestedQuestion should not be empty")
	}
}

func TestRunGetEstimationGuide_FallbackToGenericGuide(t *testing.T) {
	client := &fakePricingClient{allServicesErr: errors.New("api down")}
	freeTier := &fakeFreeTierProvider{err: errors.New("not found")}

	out, err := runGetEstimationGuide(context.Background(), client, freeTier, GetEstimationGuideInput{ServiceName: "Unknown Service"})
	if err != nil {
		t.Fatalf("runGetEstimationGuide returned error: %v", err)
	}
	if out.Guide.ServiceDescription != "Dynamic guide not available - using generic GCP service estimation template" {
		t.Errorf("expected generic guide, got %q", out.Guide.ServiceDescription)
	}
	if out.Guide.FreeTier == nil || out.Guide.FreeTier.Available {
		t.Errorf("FreeTier.Available should be false: %+v", out.Guide.FreeTier)
	}
}

func TestRunGetEstimationGuide_EmptyServiceName(t *testing.T) {
	client := &fakePricingClient{}

	if _, err := runGetEstimationGuide(context.Background(), client, nil, GetEstimationGuideInput{}); err == nil {
		t.Fatal("runGetEstimationGuide should return error when service_name is empty")
	}
}

func hasParameter(params []RequiredParameter, name string) bool {
	for _, p := range params {
		if p.Name == name {
			return true
		}
	}
	return false
}
