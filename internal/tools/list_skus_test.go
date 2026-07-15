package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

func regionalSKU(id, name, region string) pricing.SKU {
	return pricing.SKU{
		SKUID:       id,
		DisplayName: name,
		GeoTaxonomy: pricing.GeoTaxonomy{
			RegionalMetadata: pricing.RegionalMetadata{Region: pricing.Region{Region: region}},
		},
	}
}

func TestRunListSKUs_NoFilters(t *testing.T) {
	client := &fakePricingClient{
		listSKUsResp: &pricing.ListSKUsResponse{
			SKUs:          []pricing.SKU{regionalSKU("K1", "N1 Core", "us-central1")},
			NextPageToken: "next",
		},
	}

	out, err := runListSKUs(context.Background(), client, ListSKUsInput{ServiceID: "SVC"})
	if err != nil {
		t.Fatalf("runListSKUs returned error: %v", err)
	}
	if out.TotalReturned != 1 || out.NextPageToken != "next" || out.ServiceID != "SVC" {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestRunListSKUs_RegionFilter(t *testing.T) {
	client := &fakePricingClient{
		allSKUs: []pricing.SKU{
			regionalSKU("K1", "N1 Core", "us-central1"),
			regionalSKU("K2", "N1 Core", "asia-northeast1"),
		},
	}

	out, err := runListSKUs(context.Background(), client, ListSKUsInput{ServiceID: "SVC", Region: "asia-northeast1"})
	if err != nil {
		t.Fatalf("runListSKUs returned error: %v", err)
	}
	if out.TotalReturned != 1 || out.SKUs[0].SKUID != "K2" {
		t.Errorf("unexpected SKUs: %+v", out.SKUs)
	}
}

func TestRunListSKUs_EmptyServiceID(t *testing.T) {
	client := &fakePricingClient{}

	if _, err := runListSKUs(context.Background(), client, ListSKUsInput{}); err == nil {
		t.Fatal("runListSKUs should return error when service_id is empty")
	}
}

func TestRunListSKUs_APIError(t *testing.T) {
	client := &fakePricingClient{allSKUsErr: errors.New("api down")}

	if _, err := runListSKUs(context.Background(), client, ListSKUsInput{ServiceID: "SVC", Keyword: "core"}); err == nil {
		t.Fatal("runListSKUs should return error when API fails")
	}
}
