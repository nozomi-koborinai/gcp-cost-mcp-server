package tools

import (
	"context"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// fakePricingClient is a hand-written stub for PricingClient. Each method
// returns the corresponding fields. Optional *By* maps override the single
// response fields when present (useful for multi-service scenarios).
type fakePricingClient struct {
	listServicesResp  *pricing.ListServicesResponse
	listServicesErr   error
	listSKUsResp      *pricing.ListSKUsResponse
	listSKUsErr       error
	listSKUsByService map[string]*pricing.ListSKUsResponse
	priceResp         *pricing.GetPriceResponse
	priceErr          error
	priceBySKU        map[string]*pricing.GetPriceResponse
	allServices       []pricing.Service
	allServicesErr    error
	allSKUs           []pricing.SKU
	allSKUsByService  map[string][]pricing.SKU
	allSKUsErr        error
}

func (f *fakePricingClient) ListServices(ctx context.Context, pageSize int, pageToken string) (*pricing.ListServicesResponse, error) {
	return f.listServicesResp, f.listServicesErr
}

func (f *fakePricingClient) ListSKUs(ctx context.Context, serviceID string, pageSize int, pageToken string) (*pricing.ListSKUsResponse, error) {
	if f.listSKUsByService != nil {
		if resp, ok := f.listSKUsByService[serviceID]; ok {
			return resp, f.listSKUsErr
		}
	}
	return f.listSKUsResp, f.listSKUsErr
}

func (f *fakePricingClient) GetSKUPrice(ctx context.Context, skuID, currencyCode string) (*pricing.GetPriceResponse, error) {
	if f.priceBySKU != nil {
		if resp, ok := f.priceBySKU[skuID]; ok {
			return resp, f.priceErr
		}
	}
	return f.priceResp, f.priceErr
}

func (f *fakePricingClient) ListAllServices(ctx context.Context) ([]pricing.Service, error) {
	return f.allServices, f.allServicesErr
}

func (f *fakePricingClient) ListAllSKUs(ctx context.Context, serviceID string) ([]pricing.SKU, error) {
	if f.allSKUsByService != nil {
		if skus, ok := f.allSKUsByService[serviceID]; ok {
			return skus, f.allSKUsErr
		}
	}
	return f.allSKUs, f.allSKUsErr
}

// fakeFreeTierProvider is a hand-written stub for FreeTierProvider.
type fakeFreeTierProvider struct {
	info          *freetier.FreeTierInfo
	infoByService map[string]*freetier.FreeTierInfo
	err           error
}

func (f *fakeFreeTierProvider) GetFreeTier(ctx context.Context, serviceName string) (*freetier.FreeTierInfo, error) {
	if f.infoByService != nil {
		if info, ok := f.infoByService[serviceName]; ok {
			return info, nil
		}
	}
	return f.info, f.err
}
