// GCP Cost MCP Server
//
// A Model Context Protocol (MCP) server that provides Google Cloud cost estimation
// capabilities using the Cloud Billing Pricing API.
//
// Features:
//   - List all Google Cloud services
//   - List SKUs for specific services
//   - Get pricing information for SKUs
//   - Estimate costs based on usage
//
// Authentication:
//
//	Uses Application Default Credentials (ADC). Set up with:
//	  gcloud auth application-default login
//
// Usage:
//
//	go run main.go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/freetier"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/pricing"
	"github.com/nozomi-koborinai/gcp-cost-mcp-server/internal/tools"
)

const (
	serverName    = "gcp-cost-mcp-server"
	serverVersion = "1.0.0"
)

func main() {
	// Create context that listens for interrupt signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize Genkit
	g := genkit.Init(ctx)

	// Create Pricing API client with ADC
	pricingClient, err := pricing.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create Pricing API client: %v", err)
	}

	// Create FreeTierService for free tier information retrieval
	freeTierService := freetier.NewService()
	log.Println("FreeTierService initialized with 24h cache TTL")

	// Define tools
	toolList := []ai.Tool{
		tools.NewGetEstimationGuide(g, pricingClient, freeTierService), // Should be called first to understand requirements
		tools.NewListServices(g, pricingClient),
		tools.NewListSKUs(g, pricingClient),
		tools.NewGetSKUPrice(g, pricingClient),
<<<<<<< HEAD
		tools.NewEstimateCost(g, pricingClient), // Free tier auto-apply will be added in PR #8
=======
		tools.NewEstimateCost(g, pricingClient, freeTierService), // Now includes free tier auto-apply
>>>>>>> 0aec97d (refactor: replace hardcoded guides with dynamic SKU-based generation)
	}

	// Log registered tools
	log.Printf("Registered %d tools:", len(toolList))
	for _, tool := range toolList {
		log.Printf("  - %s", tool.Name())
	}

	// Create MCP server
	server := mcp.NewMCPServer(g, mcp.MCPServerOptions{
		Name:    serverName,
		Version: serverVersion,
	})

	// Start MCP server via stdio
	log.Printf("Starting %s v%s...", serverName, serverVersion)
	log.Println("MCP Server is ready. Waiting for client connections via stdio...")

	if err := server.ServeStdio(); err != nil && err != context.Canceled {
		log.Fatalf("MCP server error: %v", err)
	}

	log.Println("Server shutdown complete.")
}
