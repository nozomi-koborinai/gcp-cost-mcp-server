package tools

import (
	"context"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing/supplemental"
)

// WithSupplemental wraps a PricingClient so Catalog-absent products
// (License Manager / Office SPLA, etc.) appear in list/price/estimate flows.
func WithSupplemental(inner PricingClient) PricingClient {
	if inner == nil {
		return nil
	}
	return &supplementalClient{inner: inner}
}

type supplementalClient struct {
	inner PricingClient
}

func (c *supplementalClient) ListServices(ctx context.Context, pageSize int, pageToken string) (*pricing.ListServicesResponse, error) {
	resp, err := c.inner.ListServices(ctx, pageSize, pageToken)
	if err != nil {
		return nil, err
	}
	// Append curated services on the final page so unfiltered pagination
	// still surfaces Catalog-absent products.
	if resp.NextPageToken == "" {
		resp.Services = mergeServices(resp.Services, supplemental.Services())
	}
	return resp, nil
}

func (c *supplementalClient) ListAllServices(ctx context.Context) ([]pricing.Service, error) {
	services, err := c.inner.ListAllServices(ctx)
	if err != nil {
		return nil, err
	}
	return mergeServices(services, supplemental.Services()), nil
}

func (c *supplementalClient) ListSKUs(ctx context.Context, serviceID string, pageSize int, pageToken string) (*pricing.ListSKUsResponse, error) {
	if supplemental.IsService(serviceID) {
		skus := supplemental.SKUsForService(serviceID)
		return &pricing.ListSKUsResponse{SKUs: skus}, nil
	}
	return c.inner.ListSKUs(ctx, serviceID, pageSize, pageToken)
}

func (c *supplementalClient) ListAllSKUs(ctx context.Context, serviceID string) ([]pricing.SKU, error) {
	if supplemental.IsService(serviceID) {
		return supplemental.SKUsForService(serviceID), nil
	}
	return c.inner.ListAllSKUs(ctx, serviceID)
}

func (c *supplementalClient) GetSKUPrice(ctx context.Context, skuID, currencyCode string) (*pricing.GetPriceResponse, error) {
	if supplemental.FindSKU(skuID) != nil {
		return supplemental.PriceResponse(skuID, currencyCode)
	}
	return c.inner.GetSKUPrice(ctx, skuID, currencyCode)
}

func mergeServices(base, extra []pricing.Service) []pricing.Service {
	seen := make(map[string]bool, len(base))
	out := make([]pricing.Service, 0, len(base)+len(extra))
	for _, svc := range base {
		seen[svc.ServiceID] = true
		out = append(out, svc)
	}
	for _, svc := range extra {
		if seen[svc.ServiceID] {
			continue
		}
		out = append(out, svc)
	}
	return out
}

// enrichSKUInfo adds supplemental billing metadata when the SKU is curated.
func enrichSKUInfo(info SKUInfo) SKUInfo {
	sku := supplemental.LookupSKUMeta(info.SKUID)
	if sku == nil {
		return info
	}
	info.BillingModel = sku.BillingModel
	info.BillingNotes = append([]string(nil), sku.BillingNotes...)
	info.SourceURL = sku.SourceURL
	return info
}

func enrichPriceInfo(info *PriceInfo) {
	sku := supplemental.LookupSKUMeta(info.SKUID)
	if sku == nil {
		return
	}
	info.BillingModel = sku.BillingModel
	info.BillingNotes = append([]string(nil), sku.BillingNotes...)
	info.SourceURL = sku.SourceURL
}

func enrichCostBreakdown(estimate *CostBreakdown) {
	sku := supplemental.LookupSKUMeta(estimate.SKUID)
	if sku == nil {
		return
	}
	estimate.BillingModel = sku.BillingModel
	estimate.BillingNotes = append([]string(nil), sku.BillingNotes...)
	estimate.SourceURL = sku.SourceURL
	if estimate.ServiceName == "" {
		if svc := supplemental.FindService(sku.ServiceID); svc != nil {
			estimate.ServiceName = svc.DisplayName
		}
	}
}

func supplementalGuide(serviceName string) *EstimationGuide {
	svc := supplemental.FindServiceByName(serviceName)
	if svc == nil {
		return nil
	}

	params := make([]RequiredParameter, len(svc.GuideParameters))
	for i, p := range svc.GuideParameters {
		params[i] = RequiredParameter{
			Name:        p.Name,
			Description: p.Description,
			Required:    p.Required,
			Examples:    append([]string(nil), p.Examples...),
			DefaultTip:  p.DefaultTip,
		}
	}

	return &EstimationGuide{
		ServiceName:        svc.DisplayName,
		ServiceID:          svc.ServiceID,
		ServiceDescription: svc.Description,
		Parameters:         params,
		PricingFactors:     append([]string(nil), svc.PricingFactors...),
		Tips:               append([]string(nil), svc.Tips...),
		AvailableRegions:   []string{"global"},
		SKUCategories:      skuCategories(svc),
	}
}

func skuCategories(svc *supplemental.Service) []string {
	seen := make(map[string]bool)
	var out []string
	for _, sku := range svc.SKUs {
		for _, c := range sku.Categories {
			if seen[c] {
				continue
			}
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// Ensure the wrapper satisfies PricingClient.
var _ PricingClient = (*supplementalClient)(nil)
