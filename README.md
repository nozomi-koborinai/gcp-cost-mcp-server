# GCP Cost MCP Server

> **Note**: This is an unofficial project and is not affiliated with, endorsed by, or supported by Google or the Genkit team.

![AI Workflow Pipeline with Genkit](assets/hero-image.jpeg)

An MCP (Model Context Protocol) server for estimating Google Cloud running costs.

Instead of manually using the [Google Cloud Pricing Calculator](https://cloud.google.com/products/calculator), you can get GCP cost estimates directly from AI assistants like Claude Desktop, Gemini CLI, or Cursor.

## Features

| Tool | Description |
|------|-------------|
| `list_services` | Lists all available Google Cloud services |
| `list_skus` | Lists SKUs (billable items) for a specific service |
| `get_sku_price` | Gets pricing details for a specific SKU (requires SKU ID from `list_skus`) |
| `estimate_cost` | Calculates cost based on usage amount (requires SKU ID from `list_skus`) |

## Quick Start

### Prerequisites

- Google Cloud SDK (`gcloud`) installed
- Application Default Credentials configured

> **Note**: No Google Cloud project setup or API enablement is required. This server accesses public pricing data using OAuth authentication.

### 1. Set up Authentication

```bash
gcloud auth application-default login
```

### 2. Install

**Option A: Download pre-built binary (Recommended)**

Download from [GitHub Releases](https://github.com/nozomi-koborinai/gcp-cost-mcp-server/releases) for your platform.

**Option B: Build from source**

```bash
git clone https://github.com/nozomi-koborinai/gcp-cost-mcp-server.git
cd gcp-cost-mcp-server
go build -o gcp-cost-mcp-server .
```

### 3. Configure Your AI Client

#### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/path/to/gcp-cost-mcp-server"
    }
  }
}
```

#### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/path/to/gcp-cost-mcp-server"
    }
  }
}
```

#### Gemini CLI

Add to `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/path/to/gcp-cost-mcp-server"
    }
  }
}
```

## Usage Examples

Ask your AI assistant questions like:

- "What's the service ID for Compute Engine?"
- "List the available SKUs for Compute Engine"
- "How much would an n1-standard-1 instance cost for 730 hours in Tokyo region?"
- "What's the monthly cost for 100GB of Cloud Storage Standard Storage?"

## Authentication Options

### Using Application Default Credentials (Recommended)

This is the simplest option for personal use:

```bash
gcloud auth application-default login
```

### Using a Service Account

For production or automated environments, use a service account:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/path/to/gcp-cost-mcp-server",
      "env": {
        "GOOGLE_APPLICATION_CREDENTIALS": "/path/to/service-account.json"
      }
    }
  }
}
```

**Required IAM Role** (only for custom/contract pricing):
- `roles/billing.viewer` (Billing Account Viewer)

> For public pricing data, no IAM roles are required.

---

## Development

### Build

```bash
go build -o gcp-cost-mcp-server .
```

### Test with MCP Inspector

```bash
npx @modelcontextprotocol/inspector ./gcp-cost-mcp-server
```

### Cross-compile

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dist/gcp-cost-mcp-server-darwin-arm64 .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dist/gcp-cost-mcp-server-darwin-amd64 .

# Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o dist/gcp-cost-mcp-server-linux-amd64 .

# Windows (x86_64)
GOOS=windows GOARCH=amd64 go build -o dist/gcp-cost-mcp-server-windows-amd64.exe .
```

## Why Genkit for Go?

This MCP server is built with [Genkit for Go](https://github.com/firebase/genkit/tree/main/go) rather than using the raw [mcp-go](https://github.com/mark3labs/mcp-go) library directly. Here's why:

### Type-Safe Tool Definitions

Genkit automatically generates JSON schemas from Go struct tags, eliminating manual schema definitions:

```go
// Genkit: Type-safe with auto-generated schema
genkit.DefineTool(g, "list_skus", "Lists SKUs for a service",
    func(ctx *ai.ToolContext, input struct {
        ServiceID string `json:"service_id" jsonschema_description:"The service ID"`
        PageSize  int    `json:"page_size,omitempty"`
    }) (*Output, error) {
        // Implementation
    })
```

### Automatic MCP Bridge

Genkit's MCP plugin automatically discovers tools from the registry and converts them to MCP format—no manual registration required.

### Unified Ecosystem

| Feature | Benefit |
|---------|---------|
| **Genkit UI** | Debug and test tools visually during development |
| **Tracing** | Automatic execution tracing and observability |
| **AI Model Integration** | Seamlessly connect with Gemini, Bedrock, OpenAI |
| **MCP Host** | Consume other MCP servers in the same codebase |

### Tool Interruption Support

Genkit's `ToolContext` provides interrupt/resume capabilities for long-running operations—useful for user confirmation flows.

### Future-Proof

The same tool definitions work as:
- MCP Server tools (for Claude Desktop, Cursor, Gemini CLI)
- Genkit Flow components (for AI agent workflows)
- HTTP API endpoints (via `genkit.Handler`)

For more details, see the [Genkit MCP Plugin documentation](https://github.com/firebase/genkit/tree/main/go/plugins/mcp).

## License

MIT License
