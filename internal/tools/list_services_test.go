package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

func TestRunListServices_NoFilters(t *testing.T) {
	client := &fakePricingClient{
		listServicesResp: &pricing.ListServicesResponse{
			Services: []pricing.Service{
				{ServiceID: "S1", DisplayName: "Cloud Run"},
				{ServiceID: "S2", DisplayName: "BigQuery"},
			},
			NextPageToken: "next",
		},
	}

	out, err := runListServices(context.Background(), client, ListServicesInput{PageSize: 50})
	if err != nil {
		t.Fatalf("runListServices returned error: %v", err)
	}
	if out.TotalReturned != 2 {
		t.Errorf("TotalReturned = %d, want 2", out.TotalReturned)
	}
	if out.NextPageToken != "next" {
		t.Errorf("NextPageToken = %q, want next (pagination must be preserved without filters)", out.NextPageToken)
	}
	if out.Services[0].ServiceID != "S1" || out.Services[0].DisplayName != "Cloud Run" {
		t.Errorf("unexpected first service: %+v", out.Services[0])
	}
}

func TestRunListServices_NameFilter(t *testing.T) {
	client := &fakePricingClient{
		allServices: []pricing.Service{
			{ServiceID: "S1", DisplayName: "Cloud Run"},
			{ServiceID: "S2", DisplayName: "BigQuery"},
			{ServiceID: "S3", DisplayName: "Cloud Run functions"},
		},
	}

	out, err := runListServices(context.Background(), client, ListServicesInput{Name: "cloud run"})
	if err != nil {
		t.Fatalf("runListServices returned error: %v", err)
	}
	if out.TotalReturned != 2 {
		t.Fatalf("TotalReturned = %d, want 2", out.TotalReturned)
	}
	if out.NextPageToken != "" {
		t.Errorf("NextPageToken = %q, want empty when filters are applied", out.NextPageToken)
	}
}

func TestRunListServices_CoreOnlyFilter(t *testing.T) {
	client := &fakePricingClient{
		allServices: []pricing.Service{
			{ServiceID: "S1", DisplayName: "Cloud Run"},
			{ServiceID: "S2", DisplayName: "OpenLogic CentOS"},
		},
	}

	out, err := runListServices(context.Background(), client, ListServicesInput{CoreOnly: true})
	if err != nil {
		t.Fatalf("runListServices returned error: %v", err)
	}
	if out.TotalReturned != 1 || out.Services[0].DisplayName != "Cloud Run" {
		t.Errorf("unexpected services: %+v", out.Services)
	}
}

func TestRunListServices_APIError(t *testing.T) {
	client := &fakePricingClient{listServicesErr: errors.New("api down")}

	if _, err := runListServices(context.Background(), client, ListServicesInput{}); err == nil {
		t.Fatal("runListServices should return error when API fails")
	}
}
