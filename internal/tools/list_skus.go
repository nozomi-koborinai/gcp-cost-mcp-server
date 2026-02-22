package tools

import (
	"fmt"
	"log"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// ListSKUsInput is the input for the list_skus tool
type ListSKUsInput struct {
	ServiceID string `json:"service_id" jsonschema_description:"The service ID to list SKUs for (e.g., '6F81-5844-456A' for Compute Engine). Use list_services to find service IDs."`
	Region    string `json:"region,omitempty" jsonschema_description:"Filter SKUs by service region (e.g., 'asia-northeast1', 'us-central1'). When specified, only SKUs available in the given region are returned."`
	Keyword   string `json:"keyword,omitempty" jsonschema_description:"Filter SKUs by display name substring match (e.g., 'Basic M1', 'N2 Custom'). Case-insensitive."`
	Category  string `json:"category,omitempty" jsonschema_description:"Filter SKUs by category (e.g., 'Compute', 'Storage', 'Network'). Case-insensitive substring match."`
	PageSize  int    `json:"page_size,omitempty" jsonschema_description:"Number of SKUs to return per page (default: 50, max: 5000). When filters are applied, all matching SKUs are returned regardless of this value."`
	PageToken string `json:"page_token,omitempty" jsonschema_description:"Token for pagination to get next page of results. Not used when filters are applied."`
}

// SKUInfo represents simplified SKU information
type SKUInfo struct {
	SKUID       string   `json:"sku_id"`
	DisplayName string   `json:"display_name"`
	Region      string   `json:"region,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

// ListSKUsOutput is the output of the list_skus tool
type ListSKUsOutput struct {
	SKUs          []SKUInfo `json:"skus"`
	NextPageToken string    `json:"next_page_token,omitempty"`
	TotalReturned int       `json:"total_returned"`
	ServiceID     string    `json:"service_id"`
}

// NewListSKUs creates a tool that lists SKUs for a specific Google Cloud service
func NewListSKUs(g *genkit.Genkit, client *pricing.Client) ai.Tool {
	return genkit.DefineTool(
		g,
		"list_skus",
		"Lists SKUs (Stock Keeping Units) for a specific Google Cloud service. Each SKU represents a billable item with its own pricing. Use the sku_id to get detailed pricing information. Supports optional filtering by region, keyword (display name), and category to quickly find specific SKUs without manual pagination.",
		func(ctx *ai.ToolContext, input ListSKUsInput) (*ListSKUsOutput, error) {
			log.Printf("Tool 'list_skus' called for service_id: %s (region=%q, keyword=%q, category=%q)",
				input.ServiceID, input.Region, input.Keyword, input.Category)

			if input.ServiceID == "" {
				return nil, fmt.Errorf("service_id is required")
			}

			hasFilters := input.Region != "" || input.Keyword != "" || input.Category != ""

			var allSKUs []pricing.SKU

			if hasFilters {
				// When filters are applied, fetch all SKUs to ensure complete results
				pageToken := ""
				for {
					resp, err := client.ListSKUs(ctx.Context, input.ServiceID, 5000, pageToken)
					if err != nil {
						log.Printf("Error listing SKUs: %v", err)
						return nil, fmt.Errorf("failed to list SKUs: %w", err)
					}
					allSKUs = append(allSKUs, resp.SKUs...)
					if resp.NextPageToken == "" {
						break
					}
					pageToken = resp.NextPageToken
				}
			} else {
				resp, err := client.ListSKUs(ctx.Context, input.ServiceID, input.PageSize, input.PageToken)
				if err != nil {
					log.Printf("Error listing SKUs: %v", err)
					return nil, fmt.Errorf("failed to list SKUs: %w", err)
				}
				allSKUs = resp.SKUs

				// When no filters, preserve API pagination
				skus := convertSKUs(allSKUs)
				return &ListSKUsOutput{
					SKUs:          skus,
					NextPageToken: resp.NextPageToken,
					TotalReturned: len(skus),
					ServiceID:     input.ServiceID,
				}, nil
			}

			skus := convertSKUs(allSKUs)
			skus = filterSKUs(skus, input.Region, input.Keyword, input.Category)

			return &ListSKUsOutput{
				SKUs:          skus,
				TotalReturned: len(skus),
				ServiceID:     input.ServiceID,
			}, nil
		})
}

func convertSKUs(raw []pricing.SKU) []SKUInfo {
	skus := make([]SKUInfo, len(raw))
	for i, sku := range raw {
		var categories []string
		for _, cat := range sku.ProductTaxonomy.TaxonomyCategories {
			categories = append(categories, cat.Category)
		}

		region := ""
		if sku.GeoTaxonomy.RegionalMetadata.Region.Region != "" {
			region = sku.GeoTaxonomy.RegionalMetadata.Region.Region
		} else if sku.GeoTaxonomy.Type == "GLOBAL" {
			region = "global"
		}

		skus[i] = SKUInfo{
			SKUID:       sku.SKUID,
			DisplayName: sku.DisplayName,
			Region:      region,
			Categories:  categories,
		}
	}
	return skus
}

func filterSKUs(skus []SKUInfo, region, keyword, category string) []SKUInfo {
	filtered := make([]SKUInfo, 0, len(skus))
	regionLower := strings.ToLower(region)
	keywordLower := strings.ToLower(keyword)
	categoryLower := strings.ToLower(category)

	for _, sku := range skus {
		if regionLower != "" && !strings.EqualFold(sku.Region, regionLower) {
			continue
		}
		if keywordLower != "" && !strings.Contains(strings.ToLower(sku.DisplayName), keywordLower) {
			continue
		}
		if categoryLower != "" {
			matched := false
			for _, cat := range sku.Categories {
				if strings.Contains(strings.ToLower(cat), categoryLower) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, sku)
	}
	return filtered
}
