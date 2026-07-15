// Package pricing provides a client for Google Cloud Billing Pricing API.
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/oauth2/google"
)

const (
	// BaseURL is the base URL for the Cloud Billing API
	BaseURL = "https://cloudbilling.googleapis.com"
	// DefaultPageSize is the default number of items per page (max: 5000)
	DefaultPageSize = 5000
)

// Client is a client for the Google Cloud Billing Pricing API
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Pricing API client using Application Default Credentials
func NewClient(ctx context.Context) (*Client, error) {
	// Use ADC to create an authenticated HTTP client
	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-billing.readonly")
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	return &Client{
		httpClient: client,
		baseURL:    BaseURL,
	}, nil
}

// getJSON performs an authenticated GET request and decodes the JSON
// response into out. Non-200 responses are returned as errors including
// the response body.
func (c *Client) getJSON(ctx context.Context, reqURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// Service represents a Google Cloud service
type Service struct {
	Name        string `json:"name"`
	ServiceID   string `json:"serviceId"`
	DisplayName string `json:"displayName"`
}

// ListServicesResponse is the response from listing services
type ListServicesResponse struct {
	Services      []Service `json:"services"`
	NextPageToken string    `json:"nextPageToken,omitempty"`
}

// SKU represents a Stock Keeping Unit
type SKU struct {
	Name            string          `json:"name"`
	SKUID           string          `json:"skuId"`
	DisplayName     string          `json:"displayName"`
	Service         string          `json:"service"`
	ProductTaxonomy ProductTaxonomy `json:"productTaxonomy,omitempty"`
	GeoTaxonomy     GeoTaxonomy     `json:"geoTaxonomy,omitempty"`
}

// ProductTaxonomy contains product categorization
type ProductTaxonomy struct {
	TaxonomyCategories []TaxonomyCategory `json:"taxonomyCategories,omitempty"`
}

// TaxonomyCategory represents a category in the taxonomy
type TaxonomyCategory struct {
	Category string `json:"category"`
}

// GeoTaxonomy contains geographic information
type GeoTaxonomy struct {
	Type             string           `json:"type,omitempty"`
	RegionalMetadata RegionalMetadata `json:"regionalMetadata,omitempty"`
	GlobalMetadata   *GlobalMetadata  `json:"globalMetadata,omitempty"`
}

// RegionalMetadata contains regional information
type RegionalMetadata struct {
	Region Region `json:"region,omitempty"`
}

// Region represents a geographic region
type Region struct {
	Region string `json:"region"`
}

// GlobalMetadata represents global pricing metadata
type GlobalMetadata struct{}

// ListSKUsResponse is the response from listing SKUs
type ListSKUsResponse struct {
	SKUs          []SKU  `json:"skus"`
	NextPageToken string `json:"nextPageToken,omitempty"`
}

// Price represents pricing information for a SKU
type Price struct {
	Name         string `json:"name"`
	CurrencyCode string `json:"currencyCode"`
	ValueType    string `json:"valueType"`
	Rate         *Rate  `json:"rate,omitempty"`
}

// Rate contains rate-based pricing information
type Rate struct {
	Tiers           []Tier          `json:"tiers,omitempty"`
	UnitInfo        UnitInfo        `json:"unitInfo,omitempty"`
	AggregationInfo AggregationInfo `json:"aggregationInfo,omitempty"`
}

// Tier represents a pricing tier
type Tier struct {
	StartAmount Amount `json:"startAmount,omitempty"`
	ListPrice   Money  `json:"listPrice,omitempty"`
}

// Amount represents a numeric amount
type Amount struct {
	Value string `json:"value,omitempty"`
}

// Float64 returns the amount as a float64. An empty value is treated as
// zero because the API omits zero-valued startAmount fields.
func (a Amount) Float64() (float64, error) {
	if a.Value == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(a.Value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", a.Value, err)
	}
	return v, nil
}

// Money represents a monetary value
type Money struct {
	CurrencyCode string `json:"currencyCode,omitempty"`
	Units        string `json:"units,omitempty"`
	Nanos        int64  `json:"nanos,omitempty"`
}

// UnitPrice returns the price as a float64, combining whole units and
// nanos. Empty units are treated as zero.
func (m Money) UnitPrice() (float64, error) {
	var units float64
	if m.Units != "" {
		u, err := strconv.ParseFloat(m.Units, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid price units %q: %w", m.Units, err)
		}
		units = u
	}
	return units + float64(m.Nanos)/1e9, nil
}

// UnitInfo contains unit information
type UnitInfo struct {
	Unit            string `json:"unit,omitempty"`
	UnitDescription string `json:"unitDescription,omitempty"`
	UnitQuantity    Amount `json:"unitQuantity,omitempty"`
}

// AggregationInfo contains aggregation information
type AggregationInfo struct {
	Level    string `json:"level,omitempty"`
	Interval string `json:"interval,omitempty"`
}

// SKUPrice represents a single SKU price entry
type SKUPrice struct {
	ConsumptionModel            string `json:"consumptionModel,omitempty"`
	ConsumptionModelDescription string `json:"consumptionModelDescription,omitempty"`
	ValueType                   string `json:"valueType,omitempty"`
	Rate                        *Rate  `json:"rate,omitempty"`
}

// GetPriceResponse is the response from getting a SKU price
type GetPriceResponse struct {
	Name         string     `json:"name"`
	CurrencyCode string     `json:"currencyCode"`
	SKUPrices    []SKUPrice `json:"skuPrices,omitempty"`
}

// ListPricesResponse is the response from listing prices
type ListPricesResponse struct {
	Prices        []Price `json:"prices"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
}

// ListServices lists all publicly available Google Cloud services
func (c *Client) ListServices(ctx context.Context, pageSize int, pageToken string) (*ListServicesResponse, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	params := url.Values{}
	params.Set("pageSize", strconv.Itoa(pageSize))
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	reqURL := fmt.Sprintf("%s/v2beta/services?%s", c.baseURL, params.Encode())

	var result ListServicesResponse
	if err := c.getJSON(ctx, reqURL, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListSKUs lists SKUs for a specific service
func (c *Client) ListSKUs(ctx context.Context, serviceID string, pageSize int, pageToken string) (*ListSKUsResponse, error) {
	if serviceID == "" {
		return nil, fmt.Errorf("serviceID is required")
	}

	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	params := url.Values{}
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("filter", fmt.Sprintf(`service="services/%s"`, serviceID))
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	reqURL := fmt.Sprintf("%s/v2beta/skus?%s", c.baseURL, params.Encode())

	var result ListSKUsResponse
	if err := c.getJSON(ctx, reqURL, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSKUPrice gets the price for a specific SKU
func (c *Client) GetSKUPrice(ctx context.Context, skuID string, currencyCode string) (*GetPriceResponse, error) {
	if skuID == "" {
		return nil, fmt.Errorf("skuID is required")
	}

	params := url.Values{}
	if currencyCode != "" {
		params.Set("currencyCode", currencyCode)
	}

	reqURL := fmt.Sprintf("%s/v2beta/skus/%s/price", c.baseURL, skuID)
	if len(params) > 0 {
		reqURL = fmt.Sprintf("%s?%s", reqURL, params.Encode())
	}

	var result GetPriceResponse
	if err := c.getJSON(ctx, reqURL, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAllServices lists all publicly available Google Cloud services,
// following pagination until every page has been fetched.
func (c *Client) ListAllServices(ctx context.Context) ([]Service, error) {
	var all []Service
	pageToken := ""
	for {
		resp, err := c.ListServices(ctx, DefaultPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.Services...)
		if resp.NextPageToken == "" {
			return all, nil
		}
		pageToken = resp.NextPageToken
	}
}

// ListAllSKUs lists all SKUs for a specific service, following pagination
// until every page has been fetched.
func (c *Client) ListAllSKUs(ctx context.Context, serviceID string) ([]SKU, error) {
	var all []SKU
	pageToken := ""
	for {
		resp, err := c.ListSKUs(ctx, serviceID, DefaultPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, resp.SKUs...)
		if resp.NextPageToken == "" {
			return all, nil
		}
		pageToken = resp.NextPageToken
	}
}

// CalculateCost calculates the estimated cost based on usage amount and
// pricing tiers. It is a pure computation and does not call the API.
func CalculateCost(rate *Rate, usageAmount float64) (float64, error) {
	if rate == nil {
		return 0, fmt.Errorf("invalid price data: rate is nil")
	}

	if len(rate.Tiers) == 0 {
		return 0, fmt.Errorf("no pricing tiers available")
	}

	var totalCost float64
	remainingUsage := usageAmount

	for i, tier := range rate.Tiers {
		startAmount, err := tier.StartAmount.Float64()
		if err != nil {
			return 0, fmt.Errorf("invalid tier start amount: %w", err)
		}

		var endAmount float64
		if i+1 < len(rate.Tiers) {
			endAmount, err = rate.Tiers[i+1].StartAmount.Float64()
			if err != nil {
				return 0, fmt.Errorf("invalid tier start amount: %w", err)
			}
		} else {
			endAmount = remainingUsage + startAmount + 1 // Use all remaining usage
		}

		tierRange := endAmount - startAmount
		if tierRange <= 0 {
			continue
		}

		usageInTier := remainingUsage
		if usageInTier > tierRange {
			usageInTier = tierRange
		}

		pricePerUnit, err := tier.ListPrice.UnitPrice()
		if err != nil {
			return 0, fmt.Errorf("invalid tier price: %w", err)
		}

		totalCost += usageInTier * pricePerUnit
		remainingUsage -= usageInTier

		if remainingUsage <= 0 {
			break
		}
	}

	return totalCost, nil
}
