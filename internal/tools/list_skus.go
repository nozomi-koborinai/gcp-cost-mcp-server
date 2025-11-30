package tools

import (
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// ListSKUsInput is the input for the list_skus tool
type ListSKUsInput struct {
	ServiceID string `json:"service_id" jsonschema_description:"The service ID to list SKUs for (e.g., '6F81-5844-456A' for Compute Engine). Use list_services to find service IDs."`
	PageSize  int    `json:"page_size,omitempty" jsonschema_description:"Number of SKUs to return per page (default: 50, max: 5000)"`
	PageToken string `json:"page_token,omitempty" jsonschema_description:"Token for pagination to get next page of results"`
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
		"Lists SKUs (Stock Keeping Units) for a specific Google Cloud service. Each SKU represents a billable item with its own pricing. Use the sku_id to get detailed pricing information.",
		func(ctx *ai.ToolContext, input ListSKUsInput) (*ListSKUsOutput, error) {
			log.Printf("Tool 'list_skus' called for service_id: %s", input.ServiceID)

			if input.ServiceID == "" {
				return nil, fmt.Errorf("service_id is required")
			}

			resp, err := client.ListSKUs(ctx.Context, input.ServiceID, input.PageSize, input.PageToken)
			if err != nil {
				log.Printf("Error listing SKUs: %v", err)
				return nil, fmt.Errorf("failed to list SKUs: %w", err)
			}

			skus := make([]SKUInfo, len(resp.SKUs))
			for i, sku := range resp.SKUs {
				// Extract categories
				var categories []string
				for _, cat := range sku.ProductTaxonomy.TaxonomyCategories {
					categories = append(categories, cat.Category)
				}

				// Extract region
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

			return &ListSKUsOutput{
				SKUs:          skus,
				NextPageToken: resp.NextPageToken,
				TotalReturned: len(skus),
				ServiceID:     input.ServiceID,
			}, nil
		})
}
