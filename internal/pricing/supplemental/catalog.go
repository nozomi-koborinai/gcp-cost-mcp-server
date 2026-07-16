// Package supplemental provides curated pricing for Google Cloud products
// that are absent from the Cloud Billing Catalog API.
//
// Prices and billing rules are sourced from official Google Cloud
// documentation and must be reviewed when docs change.
package supplemental

import (
	"fmt"
	"math"
	"strings"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

const (
	// ServiceIDLicenseManager is the synthetic service ID for License Manager.
	// It is not a Cloud Billing Catalog service ID.
	ServiceIDLicenseManager = "SUPPLEMENTAL-LICENSE-MANAGER"

	// SKUIDOfficeLTSC2021ProPlus is the synthetic SKU for Office SPLA pricing.
	SKUIDOfficeLTSC2021ProPlus = "SUPPLEMENTAL-OFFICE-LTSC-2021-PROPLUS"

	// BillingModelExistence means cost accrues from the resource existing
	// (e.g. license_count), not from metered runtime usage.
	BillingModelExistence = "existence"

	officeSourceURL = "https://cloud.google.com/compute/docs/instances/windows/ms-office"
)

// SKU is a curated billable item outside the Cloud Billing Catalog.
type SKU struct {
	SKUID           string
	DisplayName     string
	ServiceID       string
	ProductIDs      []string
	Region          string
	Categories      []string
	PricePerUnit    float64
	CurrencyCode    string
	Unit            string
	UnitDescription string
	BillingModel    string
	BillingNotes    []string
	SourceURL       string
	Aliases         []string
}

// Service is a curated service outside the Cloud Billing Catalog.
type Service struct {
	ServiceID       string
	DisplayName     string
	Description     string
	Aliases         []string
	SKUs            []SKU
	GuideParameters []GuideParameter
	PricingFactors  []string
	Tips            []string
}

// GuideParameter is a required estimation parameter for a supplemental service.
type GuideParameter struct {
	Name        string
	Description string
	Required    bool
	Examples    []string
	DefaultTip  string
}

// catalog is the curated set of Catalog-absent products.
// Keep entries minimal and document-backed.
var catalog = []Service{
	{
		ServiceID:   ServiceIDLicenseManager,
		DisplayName: "License Manager (Microsoft Office SPLA)",
		Description: "Google Cloud License Manager for third-party per-user licenses (e.g. Microsoft Office SPLA). Pricing is documented by Google Cloud but is not currently exposed via the Cloud Billing Catalog API.",
		Aliases: []string{
			"license manager",
			"office",
			"microsoft office",
			"office spla",
			"spla",
			"office ltsc",
		},
		SKUs: []SKU{
			{
				SKUID:           SKUIDOfficeLTSC2021ProPlus,
				DisplayName:     "Microsoft Office LTSC 2021 Professional Plus (per user / month)",
				ServiceID:       ServiceIDLicenseManager,
				ProductIDs:      []string{"Office2021ProfessionalPlus"},
				Region:          "global",
				Categories:      []string{"License", "Microsoft Office", "SPLA"},
				PricePerUnit:    21.40,
				CurrencyCode:    "USD",
				Unit:            "mo",
				UnitDescription: "user / month",
				BillingModel:    BillingModelExistence,
				BillingNotes: []string{
					"Billed by authorized license count (existence), not by VM runtime or actual Office usage.",
					"Billing starts when a License Configuration is created; charges are not prorated within the calendar month.",
					"Reducing or deactivating licenses takes effect in the next calendar month; increases apply immediately.",
					"Terraform resource google_license_manager_configuration bills via license_count.",
					"List price sourced from Google Cloud docs; may change — verify SourceURL.",
				},
				SourceURL: officeSourceURL,
				Aliases: []string{
					"office",
					"microsoft office",
					"office spla",
					"proplus",
					"ltsc 2021",
				},
			},
		},
		GuideParameters: []GuideParameter{
			{
				Name:        "license_count",
				Description: "Number of authorized users (licenses) in the License Configuration. You are billed for this count regardless of whether users actually open Office.",
				Required:    true,
				Examples:    []string{"1", "10", "50"},
				DefaultTip:  "Match google_license_manager_configuration.license_count / authorized users.",
			},
			{
				Name:        "product",
				Description: "License Manager product. Currently documented: Microsoft Office LTSC 2021 Professional Plus (SPLA product_id ProPlusSPLA2021Volume).",
				Required:    true,
				Examples:    []string{"Office LTSC 2021 Professional Plus"},
			},
		},
		PricingFactors: []string{
			"Authorized user count (license_count)",
			"Calendar-month billing (no mid-month prorating)",
			"Existence-based: creating the configuration starts billing",
		},
		Tips: []string{
			"This product is absent from the Cloud Billing Catalog API; prices come from official Google Cloud documentation.",
			"Set license_count carefully before creating the configuration — billing for the current month starts immediately.",
			"usage_amount for estimate_cost should be the number of authorized users (licenses), not VM hours.",
			"See " + officeSourceURL,
		},
	},
}

// Services returns all supplemental services as pricing.Service values.
func Services() []pricing.Service {
	out := make([]pricing.Service, len(catalog))
	for i, svc := range catalog {
		out[i] = pricing.Service{
			Name:        "services/" + svc.ServiceID,
			ServiceID:   svc.ServiceID,
			DisplayName: svc.DisplayName,
		}
	}
	return out
}

// IsService reports whether serviceID is a supplemental service.
func IsService(serviceID string) bool {
	return FindService(serviceID) != nil
}

// FindService returns the supplemental service for serviceID, or nil.
func FindService(serviceID string) *Service {
	for i := range catalog {
		if catalog[i].ServiceID == serviceID {
			svc := catalog[i]
			return &svc
		}
	}
	return nil
}

// FindServiceByName returns a supplemental service matching name or alias.
func FindServiceByName(name string) *Service {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return nil
	}
	for i := range catalog {
		svc := &catalog[i]
		if strings.ToLower(svc.DisplayName) == normalized {
			return cloneService(svc)
		}
		for _, alias := range svc.Aliases {
			if alias == normalized || strings.Contains(normalized, alias) || strings.Contains(alias, normalized) {
				return cloneService(svc)
			}
		}
	}
	return nil
}

// SKUsForService returns pricing.SKU values for a supplemental service.
func SKUsForService(serviceID string) []pricing.SKU {
	svc := FindService(serviceID)
	if svc == nil {
		return nil
	}
	out := make([]pricing.SKU, len(svc.SKUs))
	for i, sku := range svc.SKUs {
		out[i] = toPricingSKU(sku)
	}
	return out
}

// FindSKU returns the supplemental SKU for skuID, or nil.
func FindSKU(skuID string) *SKU {
	for i := range catalog {
		for j := range catalog[i].SKUs {
			if catalog[i].SKUs[j].SKUID == skuID {
				sku := catalog[i].SKUs[j]
				return &sku
			}
		}
	}
	return nil
}

// FindSKUByProductID returns the supplemental SKU for a product exposed by
// the service API (for example, Office2021ProfessionalPlus).
func FindSKUByProductID(serviceID, productID string) *SKU {
	normalized := strings.TrimSpace(productID)
	if slash := strings.LastIndex(normalized, "/"); slash >= 0 {
		normalized = normalized[slash+1:]
	}
	for i := range catalog {
		if catalog[i].ServiceID != serviceID {
			continue
		}
		for j := range catalog[i].SKUs {
			for _, candidate := range catalog[i].SKUs[j].ProductIDs {
				if strings.EqualFold(candidate, normalized) {
					sku := catalog[i].SKUs[j]
					return &sku
				}
			}
		}
	}
	return nil
}

// ProductIDsForService returns product IDs with supplemental pricing.
func ProductIDsForService(serviceID string) []string {
	svc := FindService(serviceID)
	if svc == nil {
		return nil
	}
	var productIDs []string
	for _, sku := range svc.SKUs {
		productIDs = append(productIDs, sku.ProductIDs...)
	}
	return productIDs
}

// LookupSKUMeta returns supplemental SKU metadata when skuID is curated.
func LookupSKUMeta(skuID string) *SKU {
	return FindSKU(skuID)
}

// PriceResponse builds a GetPriceResponse for a supplemental SKU.
// Only USD list prices are curated; other currency codes return an error.
func PriceResponse(skuID, currencyCode string) (*pricing.GetPriceResponse, error) {
	sku := FindSKU(skuID)
	if sku == nil {
		return nil, fmt.Errorf("supplemental SKU not found: %s", skuID)
	}

	currency := currencyCode
	if currency == "" {
		currency = "USD"
	}
	if !strings.EqualFold(currency, sku.CurrencyCode) {
		return nil, fmt.Errorf(
			"supplemental SKU %s only has a documented list price in %s (requested %s); see %s",
			sku.SKUID, sku.CurrencyCode, currency, sku.SourceURL,
		)
	}

	units, nanos := splitMoney(sku.PricePerUnit)
	return &pricing.GetPriceResponse{
		Name:         "skus/" + sku.SKUID + "/price",
		CurrencyCode: sku.CurrencyCode,
		SKUPrices: []pricing.SKUPrice{
			{
				ConsumptionModel:            "on-demand",
				ConsumptionModelDescription: "Documented list price (Cloud Billing Catalog absent)",
				ValueType:                   "rate",
				Rate: &pricing.Rate{
					Tiers: []pricing.Tier{
						{
							StartAmount: pricing.Amount{Value: "0"},
							ListPrice: pricing.Money{
								CurrencyCode: sku.CurrencyCode,
								Units:        units,
								Nanos:        nanos,
							},
						},
					},
					UnitInfo: pricing.UnitInfo{
						Unit:            sku.Unit,
						UnitDescription: sku.UnitDescription,
					},
					AggregationInfo: pricing.AggregationInfo{
						Level:    "ACCOUNT",
						Interval: "MONTHLY",
					},
				},
			},
		},
	}, nil
}

func toPricingSKU(sku SKU) pricing.SKU {
	categories := make([]pricing.TaxonomyCategory, len(sku.Categories))
	for i, c := range sku.Categories {
		categories[i] = pricing.TaxonomyCategory{Category: c}
	}

	geo := pricing.GeoTaxonomy{Type: "GLOBAL"}
	if sku.Region != "" && !strings.EqualFold(sku.Region, "global") {
		geo = pricing.GeoTaxonomy{
			Type: "REGIONAL",
			RegionalMetadata: pricing.RegionalMetadata{
				Region: pricing.Region{Region: sku.Region},
			},
		}
	}

	return pricing.SKU{
		Name:        "skus/" + sku.SKUID,
		SKUID:       sku.SKUID,
		DisplayName: sku.DisplayName,
		Service:     "services/" + sku.ServiceID,
		ProductTaxonomy: pricing.ProductTaxonomy{
			TaxonomyCategories: categories,
		},
		GeoTaxonomy: geo,
	}
}

func cloneService(svc *Service) *Service {
	if svc == nil {
		return nil
	}
	cp := *svc
	return &cp
}

// splitMoney converts a decimal dollar amount into Cloud Billing Money fields.
func splitMoney(amount float64) (units string, nanos int64) {
	if amount < 0 {
		amount = 0
	}
	whole := math.Floor(amount)
	frac := amount - whole
	// Round nanos to avoid float residue (e.g. 0.40 -> 400000000).
	n := int64(math.Round(frac * 1e9))
	if n >= 1e9 {
		whole++
		n = 0
	}
	return fmt.Sprintf("%.0f", whole), n
}
