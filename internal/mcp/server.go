// Package mcp provides a wrapper for creating MCP servers with Genkit.
package mcp

import (
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// NewMCPServer creates a new MCP server that exposes tools via the Model Context Protocol.
// This is a convenience wrapper around mcp.NewMCPServer that provides
// a simplified interface for creating MCP servers.
//
// Parameters:
//   - g: The Genkit instance to use for server creation
//   - name: A unique identifier for this MCP server instance
//   - version: Version string for the server (e.g., "1.0.0")
//   - tools: Optional slice of tools to expose. If nil, all defined tools are auto-exposed
//
// Returns:
//   - *mcp.GenkitMCPServer: The configured MCP server instance
func NewMCPServer(g *genkit.Genkit, name string, version string, tools []ai.Tool) *mcp.GenkitMCPServer {
	// Set default version if empty
	if version == "" {
		version = "1.0.0"
	}

	// Create server options
	options := mcp.MCPServerOptions{
		Name:    name,
		Version: version,
	}

	// Log exposed tools
	for _, tool := range tools {
		log.Printf("Exposing tool: %s", tool.Name())
	}

	// Create and return the MCP server
	return mcp.NewMCPServer(g, options)
}

