package tools

import (
	"context"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// fakePricingClient is a hand-written stub for PricingClient. Each method
// returns the corresponding fields.
type fakePricingClient struct {
	listServicesResp *pricing.ListServicesResponse
	listServicesErr  error
	listSKUsResp     *pricing.ListSKUsResponse
	listSKUsErr      error
	priceResp        *pricing.GetPriceResponse
	priceErr         error
	allServices      []pricing.Service
	allServicesErr   error
	allSKUs          []pricing.SKU
	allSKUsErr       error
}

func (f *fakePricingClient) ListServices(ctx context.Context, pageSize int, pageToken string) (*pricing.ListServicesResponse, error) {
	return f.listServicesResp, f.listServicesErr
}

func (f *fakePricingClient) ListSKUs(ctx context.Context, serviceID string, pageSize int, pageToken string) (*pricing.ListSKUsResponse, error) {
	return f.listSKUsResp, f.listSKUsErr
}

func (f *fakePricingClient) GetSKUPrice(ctx context.Context, skuID, currencyCode string) (*pricing.GetPriceResponse, error) {
	return f.priceResp, f.priceErr
}

func (f *fakePricingClient) ListAllServices(ctx context.Context) ([]pricing.Service, error) {
	return f.allServices, f.allServicesErr
}

func (f *fakePricingClient) ListAllSKUs(ctx context.Context, serviceID string) ([]pricing.SKU, error) {
	return f.allSKUs, f.allSKUsErr
}

// fakeFreeTierProvider is a hand-written stub for FreeTierProvider.
type fakeFreeTierProvider struct {
	info *freetier.FreeTierInfo
	err  error
}

func (f *fakeFreeTierProvider) GetFreeTier(ctx context.Context, serviceName string) (*freetier.FreeTierInfo, error) {
	return f.info, f.err
}
