// Package tools provides MCP tools for Google Cloud cost estimation.
package tools

import (
	"fmt"
	"log"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
)

// ListServicesInput is the input for the list_services tool
type ListServicesInput struct {
	Name      string `json:"name,omitempty" jsonschema_description:"Filter services by display name (case-insensitive substring match, e.g., 'Cloud Run', 'BigQuery'). When specified, all services are searched and pagination is not used."`
	CoreOnly  bool   `json:"core_only,omitempty" jsonschema_description:"If true, return only core GCP services, excluding third-party marketplace products (e.g., VM images like 'OpenLogic CentOS'). Defaults to false."`
	PageSize  int    `json:"page_size,omitempty" jsonschema_description:"Number of services to return per page (default: 50, max: 5000). Not used when name or core_only filters are applied."`
	PageToken string `json:"page_token,omitempty" jsonschema_description:"Token for pagination to get next page of results. Not used when name or core_only filters are applied."`
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
		"Lists publicly available Google Cloud services with their IDs and display names. Supports filtering by name and excluding third-party marketplace products. Use the service_id to query SKUs for a specific service.",
		func(ctx *ai.ToolContext, input ListServicesInput) (*ListServicesOutput, error) {
			log.Printf("Tool 'list_services' called with page_size: %d, name=%q, core_only=%v",
				input.PageSize, input.Name, input.CoreOnly)

			hasFilters := input.Name != "" || input.CoreOnly

			if hasFilters {
				var allServices []pricing.Service
				pageToken := ""
				for {
					resp, err := client.ListServices(ctx.Context, 5000, pageToken)
					if err != nil {
						log.Printf("Error listing services: %v", err)
						return nil, fmt.Errorf("failed to list services: %w", err)
					}
					allServices = append(allServices, resp.Services...)
					if resp.NextPageToken == "" {
						break
					}
					pageToken = resp.NextPageToken
				}

				services := filterServices(allServices, input.Name, input.CoreOnly)
				return &ListServicesOutput{
					Services:      services,
					TotalReturned: len(services),
				}, nil
			}

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

var gcpCoreServicePrefixes = []string{
	"cloud ", "google ", "compute engine", "app engine", "bigquery",
	"firebase", "anthos", "apigee", "chronicle", "looker",
	"kubernetes engine", "container ", "dataflow", "dataproc",
	"datastore", "firestore", "spanner", "bigtable", "memorystore",
	"pub/sub", "artifact registry", "secret manager", "identity ",
	"network ", "vpc ", "certificate ", "security ", "web risk",
	"recaptcha", "access ", "binary authorization", "beyondcorp",
	"dialogflow", "vertex ai", "ai platform", "automl",
	"speech-to-text", "text-to-speech", "translation", "vision ai",
	"natural language", "video intelligence", "document ai",
	"recommendations ai", "talent solution", "retail",
	"healthcare", "life sciences", "genomics",
	"iot ", "maps ", "workspace",
	"migrate ", "transfer ", "storage",
	"logging", "monitoring", "trace", "debugger", "profiler",
	"error reporting", "cloud deploy", "cloud build",
	"source repositories", "cloud tasks", "cloud scheduler",
	"workflows", "eventarc", "api gateway", "endpoints",
	"service ", "traffic director", "cloud armor",
	"cloud cdn", "cloud dns", "cloud nat", "cloud vpn",
	"cloud interconnect", "cloud router", "load balancing",
	"cloud sql", "alloydb", "bare metal",
	"vmware engine", "sole-tenant", "batch",
	"cloud composer", "data catalog", "data fusion",
	"data loss prevention", "dataplex", "datastream",
	"cloud run", "cloud functions",
	"filestore", "persistent disk", "local ssd",
	"cloud key management", "cloud hsm",
	"cloud ids", "cloud ngfw",
	"assured workloads", "org policy",
	"resource manager", "cloud billing",
	"support", "premium support",
	"carbon ", "active assist", "recommender",
	"backup and dr", "cloud console",
	"gemini",
}

func isCoreGCPService(displayName string) bool {
	nameLower := strings.ToLower(displayName)
	for _, prefix := range gcpCoreServicePrefixes {
		if strings.HasPrefix(nameLower, prefix) || strings.Contains(nameLower, prefix) {
			return true
		}
	}
	return false
}

func filterServices(raw []pricing.Service, name string, coreOnly bool) []ServiceInfo {
	var services []ServiceInfo
	nameLower := strings.ToLower(name)

	for _, svc := range raw {
		if coreOnly && !isCoreGCPService(svc.DisplayName) {
			continue
		}
		if nameLower != "" && !strings.Contains(strings.ToLower(svc.DisplayName), nameLower) {
			continue
		}
		services = append(services, ServiceInfo{
			ServiceID:   svc.ServiceID,
			DisplayName: svc.DisplayName,
		})
	}
	return services
}
