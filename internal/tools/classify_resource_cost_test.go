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

	definition := tool.Definition()
	properties, ok := definition.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("input schema properties = %#v, want object", definition.InputSchema["properties"])
	}
	wantTypes := map[string]string{
		"resource_type": "string",
		"product":       "string",
		"license_count": "number",
		"active":        "boolean",
		"currency_code": "string",
	}
	for name, wantType := range wantTypes {
		property, ok := properties[name].(map[string]any)
		if !ok {
			t.Fatalf("input schema property %q is missing: %#v", name, properties[name])
		}
		if gotType := property["type"]; gotType != wantType {
			t.Fatalf("input schema property %q type = %v, want %s", name, gotType, wantType)
		}
	}
}

func TestRunClassifyResourceCost_LicenseManagerEstimate(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})

	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration.office",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(10),
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
	if got.EstimateBasis != "authorized_count_baseline" {
		t.Fatalf("EstimateBasis = %q, want authorized_count_baseline", got.EstimateBasis)
	}
	if got.EstimatedBaseline == nil || *got.EstimatedBaseline != 214 {
		t.Fatalf("EstimatedBaseline = %v, want 214", got.EstimatedBaseline)
	}
	if got.SourceURL == "" || len(got.BillingNotes) == 0 {
		t.Fatalf("expected source and billing notes, got source=%q notes=%v", got.SourceURL, got.BillingNotes)
	}
	if len(got.MissingAttributes) != 0 ||
		!warningsContain(got.Warnings, "authorized-count baseline") {
		t.Fatalf("unexpected missing attributes or warnings: %v / %v", got.MissingAttributes, got.Warnings)
	}
}

func TestRunClassifyResourceCost_TerraformAddress(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: `module.desktop.google_license_manager_configuration.office["primary"]`,
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(2),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	if out.Classification.ResourceType != "google_license_manager_configuration" {
		t.Fatalf("ResourceType = %q", out.Classification.ResourceType)
	}
	if out.Classification.EstimatedBaseline == nil || *out.Classification.EstimatedBaseline != 42.8 {
		t.Fatalf("EstimatedBaseline = %v, want 42.8", out.Classification.EstimatedBaseline)
	}
}

func TestRunClassifyResourceCost_DoesNotMatchIndexKeyOrModuleName(t *testing.T) {
	resourceAddresses := []string{
		`google_compute_instance.vm["google_license_manager_configuration"]`,
		"module.google_license_manager_configuration.google_compute_instance.vm",
	}
	for _, resourceAddress := range resourceAddresses {
		out, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{
			ResourceType: resourceAddress,
		})
		if err != nil {
			t.Fatalf("runClassifyResourceCost(%q): %v", resourceAddress, err)
		}
		if out.Classification.Matched {
			t.Fatalf("resource address %q incorrectly matched License Manager", resourceAddress)
		}
	}
}

func TestRunClassifyResourceCost_MissingAttributesAvoidProductAssumptions(t *testing.T) {
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
	if got.BillingModel != "" || got.BillingTrigger != "" {
		t.Fatalf("product-specific billing assigned without product: model=%q trigger=%q",
			got.BillingModel, got.BillingTrigger)
	}
	if !containsString(got.MissingAttributes, "product") ||
		!containsString(got.MissingAttributes, "license_count") {
		t.Fatalf("MissingAttributes = %v, want product and license_count", got.MissingAttributes)
	}
	if !warningsContain(got.Warnings, "product-specific") {
		t.Fatalf("Warnings = %v, want product-specific billing warning", got.Warnings)
	}
}

func TestRunClassifyResourceCost_UnsupportedProductDoesNotUseOfficePrice(t *testing.T) {
	out, err := runClassifyResourceCost(context.Background(), nil, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Product:      "MicrosoftSQLServer2022Enterprise",
		LicenseCount: numberPointer(2),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched || got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/false", got.Matched, got.PricingAvailable)
	}
	if got.SKUID != "" || got.EstimatedBaseline != nil ||
		got.BillingModel != "" || got.BillingTrigger != "" {
		t.Fatalf("unsupported product received Office metadata: sku=%q estimate=%v model=%q trigger=%q",
			got.SKUID, got.EstimatedBaseline, got.BillingModel, got.BillingTrigger)
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

func TestRunClassifyResourceCost_InvalidQuantityAndInactiveSuppressEstimate(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(-1),
		Active:       boolPointer(false),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}

	got := out.Classification
	if !got.Matched || !got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/true", got.Matched, got.PricingAvailable)
	}
	if got.Quantity != nil || got.EstimatedBaseline != nil {
		t.Fatalf("invalid quantity should not produce estimate: quantity=%v estimate=%v", got.Quantity, got.EstimatedBaseline)
	}
	if !warningsContain(got.Warnings, "non-negative") ||
		!warningsContain(got.Warnings, "previous count") {
		t.Fatalf("Warnings = %v, want count and deactivation warnings", got.Warnings)
	}
}

func TestRunClassifyResourceCost_FractionalCountReturnsClassificationWarning(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(1.5),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	if !out.Classification.Matched || !out.Classification.PricingAvailable {
		t.Fatalf("classification was lost: %+v", out.Classification)
	}
	if out.Classification.Quantity != nil || out.Classification.EstimatedBaseline != nil {
		t.Fatalf("fractional count produced estimate: %+v", out.Classification)
	}
	if !warningsContain(out.Classification.Warnings, "whole number") {
		t.Fatalf("Warnings = %v, want whole-number warning", out.Classification.Warnings)
	}
}

func TestRunClassifyResourceCost_InactiveValidCountSuppressesAmbiguousBaseline(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(10),
		Active:       boolPointer(false),
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	if !out.Classification.PricingAvailable {
		t.Fatal("PricingAvailable = false, want unit price to remain available")
	}
	if out.Classification.EstimatedBaseline != nil {
		t.Fatalf("inactive resource received ambiguous baseline: %v", out.Classification.EstimatedBaseline)
	}
	if !warningsContain(out.Classification.Warnings, "previous count") {
		t.Fatalf("Warnings = %v, want current-month ambiguity warning", out.Classification.Warnings)
	}
}

func TestRunClassifyResourceCost_PricingFailurePreservesClassification(t *testing.T) {
	client := WithSupplemental(&fakePricingClient{})
	out, err := runClassifyResourceCost(context.Background(), client, ClassifyResourceCostInput{
		ResourceType: "google_license_manager_configuration",
		Product:      "Office2021ProfessionalPlus",
		LicenseCount: numberPointer(10),
		CurrencyCode: "JPY",
	})
	if err != nil {
		t.Fatalf("runClassifyResourceCost: %v", err)
	}
	got := out.Classification
	if !got.Matched || got.PricingAvailable {
		t.Fatalf("Matched/PricingAvailable = %v/%v, want true/false", got.Matched, got.PricingAvailable)
	}
	if got.SKUID != supplemental.SKUIDOfficeLTSC2021ProPlus ||
		got.BillingModel != supplemental.BillingModelExistence {
		t.Fatalf("classification lost after pricing failure: %+v", got)
	}
	if got.EstimatedBaseline != nil || !warningsContain(got.Warnings, "currency JPY") {
		t.Fatalf("unexpected estimate/warnings: %v / %v", got.EstimatedBaseline, got.Warnings)
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

func numberPointer(value float64) *float64 {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}
