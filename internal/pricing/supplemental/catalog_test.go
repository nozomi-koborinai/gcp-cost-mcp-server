package supplemental

import (
	"testing"
)

func TestServicesIncludesLicenseManager(t *testing.T) {
	services := Services()
	if len(services) == 0 {
		t.Fatal("expected at least one supplemental service")
	}
	found := false
	for _, svc := range services {
		if svc.ServiceID == ServiceIDLicenseManager {
			found = true
			if svc.DisplayName == "" {
				t.Error("License Manager display name is empty")
			}
		}
	}
	if !found {
		t.Fatalf("License Manager service %q not found", ServiceIDLicenseManager)
	}
}

func TestFindServiceByNameAliases(t *testing.T) {
	names := []string{
		"License Manager",
		"license manager",
		"Office",
		"office spla",
		"SPLA",
		"Microsoft Office",
	}
	for _, name := range names {
		svc := FindServiceByName(name)
		if svc == nil {
			t.Fatalf("FindServiceByName(%q) = nil, want License Manager", name)
		}
		if svc.ServiceID != ServiceIDLicenseManager {
			t.Fatalf("FindServiceByName(%q).ServiceID = %q, want %q", name, svc.ServiceID, ServiceIDLicenseManager)
		}
	}
}

func TestOfficeSKUPrice(t *testing.T) {
	sku := FindSKU(SKUIDOfficeLTSC2021ProPlus)
	if sku == nil {
		t.Fatal("Office SKU not found")
	}
	if sku.PricePerUnit != 21.40 {
		t.Fatalf("PricePerUnit = %v, want 21.40", sku.PricePerUnit)
	}
	if sku.BillingModel != BillingModelExistence {
		t.Fatalf("BillingModel = %q, want %q", sku.BillingModel, BillingModelExistence)
	}
	if sku.BillingTrigger != BillingTriggerConfigurationCreated {
		t.Fatalf("BillingTrigger = %q, want %q", sku.BillingTrigger, BillingTriggerConfigurationCreated)
	}
	if sku.SourceURL == "" {
		t.Fatal("SourceURL is empty")
	}
	if len(sku.ProductIDs) != 1 || sku.ProductIDs[0] != "Office2021ProfessionalPlus" {
		t.Fatalf("ProductIDs = %v, want [Office2021ProfessionalPlus]", sku.ProductIDs)
	}

	resp, err := PriceResponse(SKUIDOfficeLTSC2021ProPlus, "USD")
	if err != nil {
		t.Fatalf("PriceResponse: %v", err)
	}
	if len(resp.SKUPrices) != 1 || resp.SKUPrices[0].Rate == nil {
		t.Fatalf("unexpected price response: %+v", resp)
	}
	price, err := resp.SKUPrices[0].Rate.Tiers[0].ListPrice.UnitPrice()
	if err != nil {
		t.Fatalf("UnitPrice: %v", err)
	}
	if price != 21.40 {
		t.Fatalf("UnitPrice = %v, want 21.40", price)
	}

	if _, err := PriceResponse(SKUIDOfficeLTSC2021ProPlus, "JPY"); err == nil {
		t.Fatal("expected error for non-USD currency")
	}
}

func TestFindSKUByProductID(t *testing.T) {
	sku := FindSKUByProductID(ServiceIDLicenseManager, "Office2021ProfessionalPlus")
	if sku == nil {
		t.Fatal("FindSKUByProductID(Office2021ProfessionalPlus) = nil")
	}
	if sku.SKUID != SKUIDOfficeLTSC2021ProPlus {
		t.Fatalf("FindSKUByProductID().SKUID = %q, want %q",
			sku.SKUID, SKUIDOfficeLTSC2021ProPlus)
	}

	unsupported := []string{
		"office2021professionalplus",
		"projects/example/locations/us-central1/products/Office2021ProfessionalPlus",
		"MicrosoftSQLServer2022Enterprise",
	}
	for _, productID := range unsupported {
		if sku := FindSKUByProductID(ServiceIDLicenseManager, productID); sku != nil {
			t.Fatalf("unexpected supplemental SKU for product %q: %+v", productID, sku)
		}
	}
}

func TestProductIDsForService(t *testing.T) {
	got := ProductIDsForService(ServiceIDLicenseManager)
	if len(got) != 1 || got[0] != "Office2021ProfessionalPlus" {
		t.Fatalf("ProductIDsForService = %v, want [Office2021ProfessionalPlus]", got)
	}
}

func TestSKUsForService(t *testing.T) {
	skus := SKUsForService(ServiceIDLicenseManager)
	if len(skus) != 1 {
		t.Fatalf("SKUsForService len = %d, want 1", len(skus))
	}
	if skus[0].SKUID != SKUIDOfficeLTSC2021ProPlus {
		t.Fatalf("SKUID = %q, want %q", skus[0].SKUID, SKUIDOfficeLTSC2021ProPlus)
	}
	if skus[0].GeoTaxonomy.Type != "GLOBAL" {
		t.Fatalf("GeoTaxonomy.Type = %q, want GLOBAL", skus[0].GeoTaxonomy.Type)
	}
}

func TestSplitMoney(t *testing.T) {
	units, nanos := splitMoney(21.40)
	if units != "21" {
		t.Fatalf("units = %q, want 21", units)
	}
	if nanos != 400000000 {
		t.Fatalf("nanos = %d, want 400000000", nanos)
	}
}
