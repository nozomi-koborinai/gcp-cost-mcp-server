package tools

import (
	"context"

	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// PricingClient is the subset of the pricing API that the tools depend
// on. It is defined on the consumer side so tests can substitute a fake.
type PricingClient interface {
	ListServices(ctx context.Context, pageSize int, pageToken string) (*pricing.ListServicesResponse, error)
	ListSKUs(ctx context.Context, serviceID string, pageSize int, pageToken string) (*pricing.ListSKUsResponse, error)
	GetSKUPrice(ctx context.Context, skuID, currencyCode string) (*pricing.GetPriceResponse, error)
	ListAllServices(ctx context.Context) ([]pricing.Service, error)
	ListAllSKUs(ctx context.Context, serviceID string) ([]pricing.SKU, error)
}

// FreeTierProvider supplies free tier information for a service.
type FreeTierProvider interface {
	GetFreeTier(ctx context.Context, serviceName string) (*freetier.FreeTierInfo, error)
}

// Compile-time checks that the concrete types satisfy the interfaces.
var (
	_ PricingClient    = (*pricing.Client)(nil)
	_ FreeTierProvider = (*freetier.Service)(nil)
)
