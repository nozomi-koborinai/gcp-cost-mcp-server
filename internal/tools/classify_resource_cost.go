package tools

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing/supplemental"
)

const licenseManagerDocsURL = "https://cloud.google.com/compute/docs/instances/windows/license-manager"

// ClassifyResourceCostInput is a Terraform resource and the attributes needed
// to classify its billing behavior and, when possible, estimate its cost.
type ClassifyResourceCostInput struct {
	ResourceType string `json:"resource_type" jsonschema_description:"Terraform resource type or address (for example, google_license_manager_configuration or google_license_manager_configuration.office). REQUIRED."`
	Product      string `json:"product,omitempty" jsonschema_description:"License Manager product attribute. Currently priced: Office2021ProfessionalPlus."`
	LicenseCount *int   `json:"license_count,omitempty" jsonschema_description:"License Manager license_count attribute (authorized users or packs). Required for a baseline estimate."`
	Active       *bool  `json:"active,omitempty" jsonschema_description:"License Manager active attribute. Defaults to true in Terraform. A false value makes the current-month charge ambiguous because deactivation takes effect the following month."`
	CurrencyCode string `json:"currency_code,omitempty" jsonschema_description:"ISO-4217 currency code. Defaults to USD. Supplemental prices may only be available in their documented currency."`
}

// ResourceCostClassification describes how a Terraform resource incurs cost.
type ResourceCostClassification struct {
	ResourceType      string   `json:"resource_type"`
	Matched           bool     `json:"matched"`
	PricingAvailable  bool     `json:"pricing_available"`
	ServiceID         string   `json:"service_id,omitempty"`
	ServiceName       string   `json:"service_name,omitempty"`
	Product           string   `json:"product,omitempty"`
	SupportedProducts []string `json:"supported_products,omitempty"`
	SKUID             string   `json:"sku_id,omitempty"`
	SKUDisplayName    string   `json:"sku_display_name,omitempty"`
	BillingModel      string   `json:"billing_model,omitempty"`
	BillingTrigger    string   `json:"billing_trigger,omitempty"`
	QuantityAttribute string   `json:"quantity_attribute,omitempty"`
	Quantity          *float64 `json:"quantity,omitempty"`
	Unit              string   `json:"unit,omitempty"`
	UnitDescription   string   `json:"unit_description,omitempty"`
	EstimatePeriod    string   `json:"estimate_period,omitempty"`
	CurrencyCode      string   `json:"currency_code,omitempty"`
	PricePerUnit      *float64 `json:"price_per_unit,omitempty"`
	EstimateBasis     string   `json:"estimate_basis,omitempty"`
	EstimatedBaseline *float64 `json:"estimated_monthly_baseline,omitempty"`
	SourceURL         string   `json:"source_url,omitempty"`
	MissingAttributes []string `json:"missing_attributes,omitempty"`
	BillingNotes      []string `json:"billing_notes,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

// ClassifyResourceCostOutput is the output of classify_resource_cost.
type ClassifyResourceCostOutput struct {
	Classification ResourceCostClassification `json:"classification"`
}

const classifyResourceCostDescription = `Classifies the cost behavior of a Terraform resource and estimates an authorized-count monthly baseline when enough attributes are provided.

The tool identifies whether billing is triggered by resource existence rather than runtime usage, returns the matching service/SKU and quantity attribute, and includes source-backed billing warnings.

Currently supported:
- google_license_manager_configuration with product=Office2021ProfessionalPlus
- license_count is used as the number of authorized users

An unmatched result means no classification rule exists yet; it does NOT mean that the resource is free.`

type terraformResourceCostRule struct {
	ResourceType      string
	ServiceID         string
	ProductAttribute  string
	QuantityAttribute string
	SourceURL         string
}

var terraformResourceCostRules = []terraformResourceCostRule{
	{
		ResourceType:      "google_license_manager_configuration",
		ServiceID:         supplemental.ServiceIDLicenseManager,
		ProductAttribute:  "product",
		QuantityAttribute: "license_count",
		SourceURL:         licenseManagerDocsURL,
	},
}

// NewClassifyResourceCost creates a tool that classifies Terraform resource
// billing behavior and estimates cost when the required attributes are known.
func NewClassifyResourceCost(g *genkit.Genkit, client PricingClient) ai.Tool {
	return genkit.DefineTool(
		g,
		"classify_resource_cost",
		classifyResourceCostDescription,
		func(ctx *ai.ToolContext, input ClassifyResourceCostInput) (*ClassifyResourceCostOutput, error) {
			return runClassifyResourceCost(ctx.Context, client, input)
		})
}

func runClassifyResourceCost(ctx context.Context, client PricingClient, input ClassifyResourceCostInput) (*ClassifyResourceCostOutput, error) {
	log.Printf("Tool 'classify_resource_cost' called for resource_type: %s", input.ResourceType)

	if strings.TrimSpace(input.ResourceType) == "" {
		return nil, fmt.Errorf("resource_type is required")
	}

	rule, normalizedType := findTerraformResourceCostRule(input.ResourceType)
	classification := ResourceCostClassification{
		ResourceType: normalizedType,
		Matched:      rule != nil,
	}
	if rule == nil {
		classification.Warnings = []string{
			"No cost classification rule exists for this resource type. An unmatched result does not mean the resource is free.",
		}
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}

	classification.ServiceID = rule.ServiceID
	classification.QuantityAttribute = rule.QuantityAttribute
	classification.SupportedProducts = supplemental.ProductIDsForService(rule.ServiceID)
	classification.SourceURL = rule.SourceURL

	if svc := supplemental.FindService(rule.ServiceID); svc != nil {
		classification.ServiceName = svc.DisplayName
	}

	product := strings.TrimSpace(input.Product)
	var sku *supplemental.SKU
	if product == "" {
		classification.MissingAttributes = append(classification.MissingAttributes, rule.ProductAttribute)
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("License Manager billing rules are product-specific; provide %q to select a billing model, SKU, and price.", rule.ProductAttribute))
	} else {
		classification.Product = product
		sku = supplemental.FindSKUByProductID(rule.ServiceID, product)
		if sku == nil {
			classification.Warnings = append(classification.Warnings,
				fmt.Sprintf("No supplemental price is available for product %q. Supported products: %s.",
					product, strings.Join(classification.SupportedProducts, ", ")))
		}
	}

	switch {
	case input.LicenseCount == nil:
		classification.MissingAttributes = append(classification.MissingAttributes, rule.QuantityAttribute)
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Provide %q to calculate an authorized-count monthly baseline.", rule.QuantityAttribute))
	case *input.LicenseCount < 0:
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Attribute %q must be non-negative.", rule.QuantityAttribute))
	default:
		classification.Quantity = float64Pointer(float64(*input.LicenseCount))
	}

	inactive := input.Active != nil && !*input.Active
	if inactive {
		classification.Warnings = append(classification.Warnings,
			"active=false does not reveal the current-month charge: deactivation takes effect in the next calendar month, so the previous count and deactivation date are required.")
	}

	if sku == nil {
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}

	classification.SKUID = sku.SKUID
	classification.SKUDisplayName = sku.DisplayName
	classification.BillingModel = sku.BillingModel
	classification.BillingTrigger = sku.BillingTrigger
	classification.Unit = sku.Unit
	classification.UnitDescription = sku.UnitDescription
	classification.EstimatePeriod = "calendar_month"
	classification.SourceURL = sku.SourceURL
	classification.BillingNotes = append([]string(nil), sku.BillingNotes...)

	currencyCode := input.CurrencyCode
	if currencyCode == "" {
		currencyCode = "USD"
	}
	classification.CurrencyCode = currencyCode
	if client == nil {
		classification.Warnings = append(classification.Warnings,
			"Pricing could not be loaded because the pricing client is unavailable.")
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}

	priceResp, err := client.GetSKUPrice(ctx, sku.SKUID, currencyCode)
	if err != nil {
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Pricing could not be loaded for currency %s: %v", currencyCode, err))
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}
	rate, err := firstUsableRate(priceResp)
	if err != nil {
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Pricing data is incomplete: %v", err))
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}
	pricePerUnit, err := firstTierUnitPrice(rate)
	if err != nil {
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Pricing data is incomplete: %v", err))
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}

	classification.PricingAvailable = true
	classification.Unit = rate.UnitInfo.Unit
	classification.UnitDescription = rate.UnitInfo.UnitDescription
	classification.CurrencyCode = priceResp.CurrencyCode
	classification.PricePerUnit = float64Pointer(pricePerUnit)
	classification.EstimateBasis = "authorized_count_baseline"

	if classification.Quantity != nil && !inactive {
		estimatedCost, err := pricing.CalculateCost(rate, *classification.Quantity)
		if err != nil {
			classification.Warnings = append(classification.Warnings,
				fmt.Sprintf("The authorized-count baseline could not be calculated: %v", err))
			return &ClassifyResourceCostOutput{Classification: classification}, nil
		}
		classification.EstimatedBaseline = float64Pointer(estimatedCost)
		classification.Warnings = append(classification.Warnings,
			"This is an authorized-count baseline, not a bill forecast; current-month count reductions and user overages require billing history and usage data.")
	}

	return &ClassifyResourceCostOutput{Classification: classification}, nil
}

func findTerraformResourceCostRule(resourceTypeOrAddress string) (*terraformResourceCostRule, string) {
	resourceType := terraformResourceType(resourceTypeOrAddress)
	for i := range terraformResourceCostRules {
		rule := &terraformResourceCostRules[i]
		if resourceType == rule.ResourceType {
			return rule, rule.ResourceType
		}
	}
	return nil, resourceType
}

func terraformResourceType(resourceTypeOrAddress string) string {
	trimmed := strings.TrimSpace(resourceTypeOrAddress)
	if !strings.Contains(trimmed, ".") {
		return trimmed
	}

	withoutIndices, ok := stripTerraformIndexExpressions(trimmed)
	if !ok {
		return trimmed
	}
	parts := strings.Split(withoutIndices, ".")
	for _, part := range parts {
		if part == "" {
			return trimmed
		}
	}

	index := 0
	for index+1 < len(parts) && parts[index] == "module" {
		index += 2
	}
	// A managed resource address has exactly a type and name after any module
	// path. Matching that structural position avoids treating an index key or
	// module name as a resource type.
	if len(parts)-index == 2 {
		return parts[index]
	}
	return trimmed
}

func stripTerraformIndexExpressions(address string) (string, bool) {
	var result strings.Builder
	depth := 0
	var quote rune
	escaped := false

	for _, r := range address {
		if depth == 0 {
			if r == '[' {
				depth = 1
				continue
			}
			result.WriteRune(r)
			continue
		}

		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}

		switch r {
		case '"', '\'':
			quote = r
		case '[':
			depth++
		case ']':
			depth--
		}
	}

	if depth != 0 || quote != 0 {
		return address, false
	}
	return result.String(), true
}

func firstUsableRate(resp *pricing.GetPriceResponse) (*pricing.Rate, error) {
	if resp == nil {
		return nil, fmt.Errorf("price response is nil")
	}
	for _, skuPrice := range resp.SKUPrices {
		if skuPrice.Rate != nil {
			return skuPrice.Rate, nil
		}
	}
	return nil, fmt.Errorf("no rate is available")
}

func firstTierUnitPrice(rate *pricing.Rate) (float64, error) {
	if rate == nil || len(rate.Tiers) == 0 {
		return 0, fmt.Errorf("no pricing tiers are available")
	}
	return rate.Tiers[0].ListPrice.UnitPrice()
}

func float64Pointer(value float64) *float64 {
	return &value
}
