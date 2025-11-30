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

// Money represents a monetary value
type Money struct {
	CurrencyCode string `json:"currencyCode,omitempty"`
	Units        string `json:"units,omitempty"`
	Nanos        int64  `json:"nanos,omitempty"`
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ListServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result ListSKUsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result GetPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CalculateCost calculates the estimated cost based on usage amount and pricing tiers
func (c *Client) CalculateCost(rate *Rate, usageAmount float64) (float64, error) {
	if rate == nil {
		return 0, fmt.Errorf("invalid price data: rate is nil")
	}

	if len(rate.Tiers) == 0 {
		return 0, fmt.Errorf("no pricing tiers available")
	}

	var totalCost float64
	remainingUsage := usageAmount

	for i, tier := range rate.Tiers {
		startAmount, _ := strconv.ParseFloat(tier.StartAmount.Value, 64)

		var endAmount float64
		if i+1 < len(rate.Tiers) {
			endAmount, _ = strconv.ParseFloat(rate.Tiers[i+1].StartAmount.Value, 64)
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

		// Calculate price per unit (units + nanos)
		units, _ := strconv.ParseFloat(tier.ListPrice.Units, 64)
		nanos := float64(tier.ListPrice.Nanos) / 1e9
		pricePerUnit := units + nanos

		totalCost += usageInTier * pricePerUnit
		remainingUsage -= usageInTier

		if remainingUsage <= 0 {
			break
		}
	}

	return totalCost, nil
}
