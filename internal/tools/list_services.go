// Package tools provides MCP tools for Google Cloud cost estimation.
package tools

import (
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// ListServicesInput is the input for the list_services tool
type ListServicesInput struct {
	PageSize  int    `json:"page_size,omitempty" jsonschema_description:"Number of services to return per page (default: 50, max: 5000)"`
	PageToken string `json:"page_token,omitempty" jsonschema_description:"Token for pagination to get next page of results"`
}

// ServiceInfo represents a simplified service information
type ServiceInfo struct {
	ServiceID   string `json:"service_id"`
	DisplayName string `json:"display_name"`
}

// ListServicesOutput is the output of the list_services tool
type ListServicesOutput struct {
	Services      []ServiceInfo `json:"services"`
	NextPageToken string        `json:"next_page_token,omitempty"`
	TotalReturned int           `json:"total_returned"`
}

// NewListServices creates a tool that lists all Google Cloud services
func NewListServices(g *genkit.Genkit, client *pricing.Client) ai.Tool {
	return genkit.DefineTool(
		g,
		"list_services",
		"Lists all publicly available Google Cloud services with their IDs and display names. Use the service_id to query SKUs for a specific service.",
		func(ctx *ai.ToolContext, input ListServicesInput) (*ListServicesOutput, error) {
			log.Printf("Tool 'list_services' called with page_size: %d", input.PageSize)

			resp, err := client.ListServices(ctx.Context, input.PageSize, input.PageToken)
			if err != nil {
				log.Printf("Error listing services: %v", err)
				return nil, fmt.Errorf("failed to list services: %w", err)
			}

			services := make([]ServiceInfo, len(resp.Services))
			for i, svc := range resp.Services {
				services[i] = ServiceInfo{
					ServiceID:   svc.ServiceID,
					DisplayName: svc.DisplayName,
				}
			}

			return &ListServicesOutput{
				Services:      services,
				NextPageToken: resp.NextPageToken,
				TotalReturned: len(services),
			}, nil
		})
}
