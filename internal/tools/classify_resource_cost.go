package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
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
	ResourceType string         `json:"resource_type" jsonschema_description:"Terraform resource type or address (for example, google_license_manager_configuration or google_license_manager_configuration.office). REQUIRED."`
	Attributes   map[string]any `json:"attributes,omitempty" jsonschema_description:"Known Terraform resource attributes. For google_license_manager_configuration, provide product and license_count. Values that are unknown during planning may be omitted."`
	CurrencyCode string         `json:"currency_code,omitempty" jsonschema_description:"ISO-4217 currency code. Defaults to USD. Supplemental prices may only be available in their documented currency."`
}

// ResourceCostClassification describes how a Terraform resource incurs cost.
type ResourceCostClassification struct {
	ResourceType         string   `json:"resource_type"`
	Matched              bool     `json:"matched"`
	PricingAvailable     bool     `json:"pricing_available"`
	ServiceID            string   `json:"service_id,omitempty"`
	ServiceName          string   `json:"service_name,omitempty"`
	Product              string   `json:"product,omitempty"`
	SupportedProducts    []string `json:"supported_products,omitempty"`
	SKUID                string   `json:"sku_id,omitempty"`
	SKUDisplayName       string   `json:"sku_display_name,omitempty"`
	BillingModel         string   `json:"billing_model,omitempty"`
	BillingTrigger       string   `json:"billing_trigger,omitempty"`
	QuantityAttribute    string   `json:"quantity_attribute,omitempty"`
	Quantity             *float64 `json:"quantity,omitempty"`
	Unit                 string   `json:"unit,omitempty"`
	UnitDescription      string   `json:"unit_description,omitempty"`
	EstimatePeriod       string   `json:"estimate_period,omitempty"`
	CurrencyCode         string   `json:"currency_code,omitempty"`
	PricePerUnit         *float64 `json:"price_per_unit,omitempty"`
	EstimatedMonthlyCost *float64 `json:"estimated_monthly_cost,omitempty"`
	SourceURL            string   `json:"source_url,omitempty"`
	MissingAttributes    []string `json:"missing_attributes,omitempty"`
	BillingNotes         []string `json:"billing_notes,omitempty"`
	Warnings             []string `json:"warnings,omitempty"`
}

// ClassifyResourceCostOutput is the output of classify_resource_cost.
type ClassifyResourceCostOutput struct {
	Classification ResourceCostClassification `json:"classification"`
}

const classifyResourceCostDescription = `Classifies the cost behavior of a Terraform resource and estimates a monthly cost when enough attributes are provided.

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
	BillingModel      string
	BillingTrigger    string
	WholeUnits        bool
	SourceURL         string
}

var terraformResourceCostRules = []terraformResourceCostRule{
	{
		ResourceType:      "google_license_manager_configuration",
		ServiceID:         supplemental.ServiceIDLicenseManager,
		ProductAttribute:  "product",
		QuantityAttribute: "license_count",
		BillingModel:      supplemental.BillingModelExistence,
		BillingTrigger:    "configuration_created",
		WholeUnits:        true,
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
	classification.BillingModel = rule.BillingModel
	classification.BillingTrigger = rule.BillingTrigger
	classification.QuantityAttribute = rule.QuantityAttribute
	classification.SupportedProducts = supplemental.ProductIDsForService(rule.ServiceID)
	classification.SourceURL = rule.SourceURL

	if svc := supplemental.FindService(rule.ServiceID); svc != nil {
		classification.ServiceName = svc.DisplayName
	}

	product, productOK := stringAttribute(input.Attributes, rule.ProductAttribute)
	var sku *supplemental.SKU
	switch {
	case !productOK:
		classification.MissingAttributes = append(classification.MissingAttributes, rule.ProductAttribute)
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Provide %q to select a product-specific SKU and price.", rule.ProductAttribute))
	case product == "":
		classification.MissingAttributes = append(classification.MissingAttributes, rule.ProductAttribute)
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Attribute %q must be a product ID string.", rule.ProductAttribute))
	default:
		classification.Product = product
		sku = supplemental.FindSKUByProductID(rule.ServiceID, product)
		if sku == nil {
			classification.Warnings = append(classification.Warnings,
				fmt.Sprintf("No supplemental price is available for product %q. Supported products: %s.",
					product, strings.Join(classification.SupportedProducts, ", ")))
		}
	}

	quantity, quantityOK, quantityErr := numericAttribute(input.Attributes, rule.QuantityAttribute)
	switch {
	case quantityErr != nil:
		classification.Warnings = append(classification.Warnings, quantityErr.Error())
	case !quantityOK:
		classification.MissingAttributes = append(classification.MissingAttributes, rule.QuantityAttribute)
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Provide %q to calculate the monthly estimate.", rule.QuantityAttribute))
	case quantity < 0:
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Attribute %q must be non-negative.", rule.QuantityAttribute))
	case rule.WholeUnits && math.Trunc(quantity) != quantity:
		classification.Warnings = append(classification.Warnings,
			fmt.Sprintf("Attribute %q must be a whole number.", rule.QuantityAttribute))
	default:
		classification.Quantity = float64Pointer(quantity)
	}

	if active, ok := boolAttribute(input.Attributes, "active"); ok && !active {
		classification.Warnings = append(classification.Warnings,
			"Deactivation does not remove the charge for the current calendar month; it takes effect in the next month.")
	}

	if sku == nil {
		return &ClassifyResourceCostOutput{Classification: classification}, nil
	}
	if client == nil {
		return nil, fmt.Errorf("pricing client is required")
	}

	currencyCode := input.CurrencyCode
	if currencyCode == "" {
		currencyCode = "USD"
	}
	priceResp, err := client.GetSKUPrice(ctx, sku.SKUID, currencyCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get price for resource %s: %w", normalizedType, err)
	}
	rate, err := firstUsableRate(priceResp)
	if err != nil {
		return nil, fmt.Errorf("invalid pricing data for resource %s: %w", normalizedType, err)
	}
	pricePerUnit, err := firstTierUnitPrice(rate)
	if err != nil {
		return nil, fmt.Errorf("invalid pricing data for resource %s: %w", normalizedType, err)
	}

	classification.PricingAvailable = true
	classification.SKUID = sku.SKUID
	classification.SKUDisplayName = sku.DisplayName
	classification.Unit = rate.UnitInfo.Unit
	classification.UnitDescription = rate.UnitInfo.UnitDescription
	classification.EstimatePeriod = "calendar_month"
	classification.CurrencyCode = priceResp.CurrencyCode
	classification.PricePerUnit = float64Pointer(pricePerUnit)
	classification.SourceURL = sku.SourceURL
	classification.BillingNotes = append([]string(nil), sku.BillingNotes...)

	if classification.Quantity != nil {
		estimatedCost, err := pricing.CalculateCost(rate, *classification.Quantity)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate cost for resource %s: %w", normalizedType, err)
		}
		classification.EstimatedMonthlyCost = float64Pointer(estimatedCost)
	}

	return &ClassifyResourceCostOutput{Classification: classification}, nil
}

func findTerraformResourceCostRule(resourceTypeOrAddress string) (*terraformResourceCostRule, string) {
	trimmed := strings.TrimSpace(resourceTypeOrAddress)
	segments := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case '.', '[', ']', '"', '\'', ' ', '\t', '\n':
			return true
		default:
			return false
		}
	})

	for i := range terraformResourceCostRules {
		rule := &terraformResourceCostRules[i]
		if trimmed == rule.ResourceType {
			return rule, rule.ResourceType
		}
		for _, segment := range segments {
			if segment == rule.ResourceType {
				return rule, rule.ResourceType
			}
		}
	}
	return nil, trimmed
}

func stringAttribute(attributes map[string]any, name string) (string, bool) {
	if attributes == nil {
		return "", false
	}
	value, ok := attributes[name]
	if !ok || value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", true
	}
	return strings.TrimSpace(text), true
}

func numericAttribute(attributes map[string]any, name string) (float64, bool, error) {
	if attributes == nil {
		return 0, false, nil
	}
	value, ok := attributes[name]
	if !ok || value == nil {
		return 0, false, nil
	}

	var number float64
	var err error
	switch v := value.(type) {
	case float64:
		number = v
	case float32:
		number = float64(v)
	case int:
		number = float64(v)
	case int8:
		number = float64(v)
	case int16:
		number = float64(v)
	case int32:
		number = float64(v)
	case int64:
		number = float64(v)
	case uint:
		number = float64(v)
	case uint8:
		number = float64(v)
	case uint16:
		number = float64(v)
	case uint32:
		number = float64(v)
	case uint64:
		number = float64(v)
	case json.Number:
		number, err = v.Float64()
	case string:
		number, err = strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		err = fmt.Errorf("expected a number, got %T", value)
	}
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, true, fmt.Errorf("attribute %q must be a finite number", name)
	}
	return number, true, nil
}

func boolAttribute(attributes map[string]any, name string) (bool, bool) {
	if attributes == nil {
		return false, false
	}
	value, ok := attributes[name]
	if !ok || value == nil {
		return false, false
	}
	boolean, ok := value.(bool)
	return boolean, ok
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
