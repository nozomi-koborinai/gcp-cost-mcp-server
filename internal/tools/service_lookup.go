package tools

import (
	"context"
	"fmt"
	"strings"
)

// findServiceByName searches for a GCP service by name and returns its ID
func findServiceByName(ctx context.Context, client PricingClient, serviceName string) (string, string, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(serviceName))

	// Fetch all services from the API (follows pagination)
	services, err := client.ListAllServices(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to list services: %w", err)
	}

	// Try exact match first
	for _, svc := range services {
		if strings.ToLower(svc.DisplayName) == normalizedName {
			return svc.ServiceID, svc.DisplayName, nil
		}
	}

	// Try partial match
	for _, svc := range services {
		svcNameLower := strings.ToLower(svc.DisplayName)
		if strings.Contains(svcNameLower, normalizedName) || strings.Contains(normalizedName, svcNameLower) {
			return svc.ServiceID, svc.DisplayName, nil
		}
	}

	// Try matching common aliases
	aliases := getServiceAliases()
	if canonical, ok := aliases[normalizedName]; ok {
		for _, svc := range services {
			if strings.ToLower(svc.DisplayName) == canonical {
				return svc.ServiceID, svc.DisplayName, nil
			}
		}
	}

	return "", "", fmt.Errorf("service not found: %s", serviceName)
}

// getServiceAliases returns a map of common service name aliases to canonical names
func getServiceAliases() map[string]string {
	return map[string]string{
		"gke":                 "kubernetes engine",
		"k8s":                 "kubernetes engine",
		"gcs":                 "cloud storage",
		"bq":                  "bigquery",
		"gcf":                 "cloud functions",
		"gae":                 "app engine",
		"gce":                 "compute engine",
		"cloud run functions": "cloud functions",
		"2nd gen functions":   "cloud functions",
		"pubsub":              "pub/sub",
		"cloud pubsub":        "pub/sub",
	}
}
