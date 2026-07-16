package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing/supplemental"
)

func TestNewClassifyResourceCost_DefinesToolSchema(t *testing.T) {
	g := genkit.Init(context.Background())
	tool := NewClassifyResourceCost(g, WithSupplemental(&fakePricingClient{}))
	if tool == nil {
		t.Fatal("NewClassifyResourceCost returned nil")
	}
	if tool.Name() != "classify_resource_cost" {
		t.Fatalf("tool name = %q, want classify_resource_cost", tool.Name())
	}
}

func TestRunClassifyResourceCost_LicenseManagerEstimate(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})

	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration.office",
		Attributes: map[string]any{
			"product":       "Office2021ProfessionalPlus",
			"license_count": 10,
		},
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched || !got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/true", got.Matched, got.PricingAvailable)
	}
	if got.ResourceType != "google_license_manager_configuration" {
		t.Fatalf("ResourceType = %q", got.ResourceType)
	}
	if got.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("BillingModel = %q, want %q", got.BillingModel, supplemental.BillingModelExistence)
	}
	if got.BillingTrigger != "configuration_created" {
		t.Fatalf("BillingTrigger = %q, want configuration_created", got.BillingTrigger)
	}
	if got.QuantityAttribute != "license_count" || got.Quantity == nil || *got.Quantity != 10 {
		t.Fatalf("quantity fields = %q/%v, want license_count/10", got.QuantityAttribute, got.Quantity)
	}
	if got.SKUID != supplemental.SKUIDOfficeLTSC2021ProPlus {
		t.Fatalf("SKUID = %q, want %q", got.SKUID, supplemental.SKUIDOfficeLTSC2021ProPlus)
	}
	if got.PricePerUnit == nil || *got.PricePerUnit != 21.40 {
		t.Fatalf("PricePerUnit = %v, want 21.40", got.PricePerUnit)
	}
	if got.EstimatedMonthlyCost == nil || *got.EstimatedMonthlyCost != 214 {
		t.Fatalf("EstimatedMonthlyCost = %v, want 214", got.EstimatedMonthlyCost)
	}
	if got.SourceURL == "" || len(got.BillingNotes) == 0 {
		t.Fatalf("expected source and billing notes, got source=%q notes=%v", got.SourceURL, got.BillingNotes)
	}
	if len(got.MissingAttributes) != 0 || len(got.Warnings) != 0 {
		t.Fatalf("unexpected missing attributes or warnings: %v / %v", got.MissingAttributes, got.Warnings)
	}
}

func TestRunClassifyResourceCost_TerraformAddress(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: `module.desktop.google_license_manager_configuration.office["primary"]`,
		Attributes: map[string]any{
			"product":       "projects/p/locations/us-central1/products/Office2021ProfessionalPlus",
			"license_count": "2",
		},
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	if out.Classification.ResourceType != "google_license_manager_configuration" {
		t.Fatalf("ResourceType = %q", out.Classification.ResourceType)
	}
	if out.Classification.EstimatedMonthlyCost == nil || *out.Classification.EstimatedMonthlyCost != 42.8 {
		t.Fatalf("EstimatedMonthlyCost = %v, want 42.8", out.Classification.EstimatedMonthlyCost)
	}
}

func TestRunClassifyResourceCost_MissingAttributesStillFlagsExistenceBilling(t *testing.T) {
	out, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched {
		t.Fatal("Matched = false, want true")
	}
	if got.PricingAvailable {
		t.Fatal("PricingAvailable = true without product")
	}
	if got.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("BillingModel = %q, want existence", got.BillingModel)
	}
	if !containsString(got.MissingAttributes, "product") ||
		!containsString(got.MissingAttributes, "license_count") {
		t.Fatalf("MissingAttributes = %v, want product and license_count", got.MissingAttributes)
	}
}

func TestRunClassifyResourceCost_UnsupportedProductDoesNotUseOfficePrice(t *testing.T) {
	out, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Attributes: map[string]any{
			"product":       "MicrosoftSQLServer2022Enterprise",
			"license_count": 2,
		},
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched || got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/false", got.Matched, got.PricingAvailable)
	}
	if got.SKUID != "" || got.EstimatedMonthlyCost != nil {
		t.Fatalf("unsupported product received Office pricing: sku=%q estimate=%v", got.SKUID, got.EstimatedMonthlyCost)
	}
	if !warningsContain(got.Warnings, "No supplemental price") {
		t.Fatalf("Warnings = %v, want unsupported-price warning", got.Warnings)
	}
}

func TestRunClassifyResourceCost_UnknownIsNotReportedAsFree(t *testing.T) {
	out, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{
		ResourceType: "google_compute_disk",
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	if out.Classification.Matched {
		t.Fatal("Matched = true for unsupported resource")
	}
	if !warningsContain(out.Classification.Warnings, "does not mean the resource is free") {
		t.Fatalf("Warnings = %v, want explicit unknown-cost warning", out.Classification.Warnings)
	}
}

func TestRunClassifyResourceCost_InvalidQuantityKeepsClassification(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Attributes: map[string]any{
			"product":       "Office2021ProfessionalPlus",
			"license_count": 1.5,
			"active":        false,
		},
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched || !got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/true", got.Matched, got.PricingAvailable)
	}
	if got.Quantity != nil || got.EstimatedMonthlyCost != nil {
		t.Fatalf("invalid quantity should not produce estimate: quantity=%v estimate=%v", got.Quantity, got.EstimatedMonthlyCost)
	}
	if !warningsContain(got.Warnings, "whole number") ||
		!warningsContain(got.Warnings, "next month") {
		t.Fatalf("Warnings = %v, want whole-number and deactivation warnings", got.Warnings)
	}
}

func TestRunClassifyResourceCost_RequiresResourceType(t *testing.T) {
	if _, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{}); err == nil {
		t.Fatal("expected error for empty resource_type")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func warningsContain(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
