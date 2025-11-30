# GCP Cost MCP Server

> **Note**: This is an unofficial project and is not affiliated with, endorsed by, or supported by Google or the Genkit team.

![AI Workflow Pipeline with Genkit](assets/hero-image.jpeg)

An MCP (Model Context Protocol) server for estimating Google Cloud running costs.

Instead of manually using the [Google Cloud Pricing Calculator](https://cloud.google.com/products/calculator), you can get GCP cost estimates directly from AI assistants like Claude Desktop, Gemini CLI, or Cursor.

## Features

### Available Tools

| Tool | Description |
|------|-------------|
| `get_estimation_guide` | **Start here!** Returns required parameters and pricing factors for any GCP service |
| `list_services` | Lists all available Google Cloud services with their IDs |
| `list_skus` | Lists SKUs (billable items) for a specific service |
| `get_sku_price` | Gets pricing details for a specific SKU |
| `estimate_cost` | Calculates cost based on SKU and usage amount |

### Recommended Workflow

The tools are designed to work together in a conversational flow:

#### Single Service Estimation

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ User: "How much would Cloud Run cost for my application?"                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 1: get_estimation_guide("Cloud Run")                                   │
│         → Returns required parameters: region, vCPU, memory, billing type,  │
│           instance count, monthly usage, etc.                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 2: AI asks user for details                                            │
│         "To estimate Cloud Run costs, I need to know:                       │
│          - Region (e.g., asia-northeast1)                                   │
│          - vCPU and memory per instance                                     │
│          - Number of instances                                              │
│          - Expected monthly usage hours..."                                 │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 3: list_services → list_skus                                           │
│         → Find the correct SKU IDs for the user's requirements              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 4: estimate_cost(sku_id, usage_amount, ...)                            │
│         → Calculate and return the cost estimate                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Architecture Diagram Estimation (Multi-Service)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ User: [Uploads architecture diagram image]                                  │
│       "Please estimate the monthly cost for this architecture"              │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 1: AI analyzes the diagram                                             │
│         → Identifies: Cloud Run, Cloud SQL, Cloud Storage, Load Balancing   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 2: get_estimation_guide for EACH service                               │
│         → Collect all required parameters for all identified services       │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 3: AI asks user for details (grouped)                                  │
│         "Region: Is asia-northeast1 okay?                                   │
│          Cloud Run: vCPU/memory/instance count?                             │
│          Cloud SQL: DB type/machine type/storage?                           │
│          ..."                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 4: For EACH service: list_services → list_skus → estimate_cost         │
│         → Calculate cost for each service                                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ Step 5: Present consolidated results                                        │
│         | Service | Config | Cost |  +  Total + Optimization tips           │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Supported Services

`get_estimation_guide` works with **any Google Cloud service**:

**Services with detailed guides:**
- **Compute**: Cloud Run, Compute Engine, GKE, Cloud Functions, App Engine (Standard/Flexible)
- **Database**: Cloud SQL, Firestore, Cloud Spanner, Memorystore for Redis
- **Storage & Analytics**: Cloud Storage, BigQuery
- **Messaging**: Pub/Sub
- **Networking**: Cloud CDN, Cloud Armor
- **DevOps**: Artifact Registry, Secret Manager

**All other services**: Returns a comprehensive generic template with common parameters (region, usage type, scale, etc.) and guidance to discover specific SKUs via `list_services`/`list_skus`.

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

Choose the installation method that best fits your environment:

#### Option A: Homebrew (macOS/Linux) — Recommended

The easiest way to install on macOS or Linux:

```bash
brew tap nozomi-koborinai/tap
brew install gcp-cost-mcp-server
```

The binary will be installed to `/opt/homebrew/bin/gcp-cost-mcp-server` (Apple Silicon) or `/usr/local/bin/gcp-cost-mcp-server` (Intel/Linux).

**Upgrading to the latest version:**

```bash
# Update tap to fetch the latest Formula
brew update

# Check the available version
brew info gcp-cost-mcp-server

# Upgrade to the latest version
brew upgrade gcp-cost-mcp-server
```

> **Note**: Always run `brew update` first to ensure you get the latest version. Without it, Homebrew uses cached Formula information.

#### Option B: Download pre-built binary

Download from [GitHub Releases](https://github.com/nozomi-koborinai/gcp-cost-mcp-server/releases) for your platform:

| Binary | Platform | Architecture |
|--------|----------|--------------|
| `gcp-cost-mcp-server-darwin-arm64` | macOS | Apple Silicon (M1/M2/M3/M4) |
| `gcp-cost-mcp-server-darwin-amd64` | macOS | Intel |
| `gcp-cost-mcp-server-linux-amd64` | Linux | x86_64 |
| `gcp-cost-mcp-server-windows-amd64.exe` | Windows | x86_64 |

> **macOS users**: After downloading, remove the quarantine attribute:
> ```bash
> chmod +x gcp-cost-mcp-server-darwin-*
> xattr -d com.apple.quarantine gcp-cost-mcp-server-darwin-*
> ```

#### Option C: Build from source

Requires Go 1.21+:

```bash
git clone https://github.com/nozomi-koborinai/gcp-cost-mcp-server.git
cd gcp-cost-mcp-server
go build -o gcp-cost-mcp-server .
```

### 3. Configure Your MCP Client

Find your binary path first:

```bash
# If installed via Homebrew
which gcp-cost-mcp-server
# Output: /opt/homebrew/bin/gcp-cost-mcp-server (Apple Silicon)
#         /usr/local/bin/gcp-cost-mcp-server (Intel/Linux)
```

#### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/opt/homebrew/bin/gcp-cost-mcp-server"
    }
  }
}
```

#### Cursor

Add to `.cursor/mcp.json` in your project or global config:

```json
{
  "mcpServers": {
    "gcp-cost": {
      "command": "/opt/homebrew/bin/gcp-cost-mcp-server"
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
      "command": "/opt/homebrew/bin/gcp-cost-mcp-server"
    }
  }
}
```

> **Note**: Replace `/opt/homebrew/bin/gcp-cost-mcp-server` with your actual binary path if different.

## Usage Examples

### Basic Cost Estimation (Conversational)

Start with a simple question - the AI will guide you through the details:

```
You: "How much would Cloud Run cost for 2 instances?"

AI: "To estimate Cloud Run costs accurately, I need a few more details:
    - Region: Where will this be deployed? (e.g., asia-northeast1, us-central1)
    - vCPU: How many vCPUs per instance? (e.g., 1, 2, 4)
    - Memory: How much memory per instance? (e.g., 1 GiB, 2 GiB)
    - Billing type: Instance-based or request-based?
    - Monthly usage: Will it run 24/7 (730 hours) or less?"

You: "Tokyo region, 1 vCPU, 2GB memory, instance-based, 24/7"

AI: "Based on your requirements for 2 Cloud Run instances in Tokyo (asia-northeast1):
    - vCPU cost: $0.000024/vCPU-sec × 2 vCPU × 2,628,000 sec = $126.14/month
    - Memory cost: $0.0000025/GiB-sec × 4 GiB × 2,628,000 sec = $26.28/month
    - Total estimated: ~$152.42/month (before free tier)
    
    💡 Tip: Cloud Run has a free tier of 240,000 vCPU-seconds and 450,000 GiB-seconds per month."
```

### Explore Services and SKUs

```
You: "What's the service ID for Compute Engine?"
You: "List the available SKUs for Compute Engine in Tokyo region"
You: "How much would an n2-standard-4 instance cost for 730 hours?"
```

### Any GCP Service

Even services without pre-built guides work:

```
You: "How much would Vertex AI cost for my ML training?"

AI: "I don't have a pre-built guide for Vertex AI, but I can help estimate costs.
    To proceed, I need to know:
    1. Region: Where will this be deployed?
    2. Usage pattern: Training or prediction? What instance type?
    3. Scale: How many hours of training? How many predictions?
    
    Would you like me to explore the available SKUs for Vertex AI?"
```

### Architecture Diagram Estimation (Multi-Service)

Upload an architecture diagram and get a complete cost breakdown:

```
You: [Upload architecture diagram image]
     "Please estimate the monthly cost for this architecture"

AI: "I've analyzed your architecture diagram. Here are the GCP services identified:
     - Cloud Run (Application layer)
     - Cloud SQL (Database)
     - Cloud Storage (Static files)
     - Cloud Load Balancing (Traffic distribution)
     
     To provide an accurate estimate, I need some details:
     
     🌏 Common Settings:
     - Region: Is asia-northeast1 (Tokyo) okay?
     
     📦 Cloud Run:
     - vCPUs: 1? 2? 4?
     - Memory: 1GB? 2GB? 4GB?
     - Instance count: How many instances?
     - Uptime: 24/7 or specific hours only?
     
     🗄️ Cloud SQL:
     - Database type: MySQL? PostgreSQL?
     - Machine type: Dev (db-f1-micro)? Production (db-custom-2-4096)?
     - Storage: How many GB needed?
     - High Availability (HA): Required?"

You: "Tokyo region, Cloud Run with 1vCPU/2GB × 2 instances running 24/7,
      Cloud SQL PostgreSQL db-custom-2-4096, 100GB SSD with HA,
      Cloud Storage 50GB Standard"

AI: "Thank you! Here's your estimate:

     | Service            | Configuration                  | Monthly Cost |
     |--------------------|--------------------------------|--------------|
     | Cloud Run          | 2 × 1vCPU × 2GB (24/7)         | $152.42      |
     | Cloud SQL          | PostgreSQL, 2vCPU/4GB, 100GB HA| $178.50      |
     | Cloud Storage      | 50GB Standard                  | $1.15        |
     | Cloud Load Balancing| Base + traffic                | $18.00       |
     | **Total**          |                                | **$350.07**  |
     
     💡 Cost Optimization Tips:
     - Cloud Run: Set min instances to 0 to reduce idle costs
     - Cloud SQL: Disable HA for ~50% savings (dev environments)
     - Consider 1yr/3yr Committed Use Discounts (CUDs) for up to 57% off"
```

This workflow:
1. AI analyzes the architecture diagram to identify GCP services
2. AI asks about shared parameters (region) first, then service-specific details
3. AI calculates costs for each service using the MCP tools
4. AI presents a consolidated breakdown with total and optimization tips

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

## Architecture

### Tool Design

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           MCP Server Tools                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────┐                                                │
│  │  get_estimation_guide   │ ◄── Entry point for cost estimation            │
│  │  ─────────────────────  │     Returns: parameters, pricing factors, tips │
│  │  • 17 detailed guides   │                                                │
│  │  • Generic fallback     │                                                │
│  └───────────┬─────────────┘                                                │
│              │                                                              │
│              ▼                                                              │
│  ┌─────────────────────────┐     ┌─────────────────────────┐               │
│  │     list_services       │────►│       list_skus         │               │
│  │  ─────────────────────  │     │  ─────────────────────  │               │
│  │  Returns: service IDs   │     │  Returns: SKU IDs,      │               │
│  │  (e.g., 6F81-5844-456A) │     │  names, regions         │               │
│  └─────────────────────────┘     └───────────┬─────────────┘               │
│                                              │                              │
│                                              ▼                              │
│                              ┌─────────────────────────┐                    │
│                              │     get_sku_price       │                    │
│                              │  ─────────────────────  │                    │
│                              │  Returns: price/unit,   │                    │
│                              │  tiers, currency        │                    │
│                              └───────────┬─────────────┘                    │
│                                          │                                  │
│                                          ▼                                  │
│                              ┌─────────────────────────┐                    │
│                              │     estimate_cost       │                    │
│                              │  ─────────────────────  │                    │
│                              │  Input: SKU ID, usage,  │                    │
│                              │  region, description    │                    │
│                              │  Returns: cost estimate │                    │
│                              └─────────────────────────┘                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
                    ┌─────────────────────────────────────┐
                    │   Google Cloud Billing API v2beta   │
                    │   cloudbilling.googleapis.com       │
                    └─────────────────────────────────────┘
```

### Data Flow

1. **get_estimation_guide**: Provides a structured guide for what information to gather
   - Detailed guides for 17 common services (Cloud Run, GKE, BigQuery, etc.)
   - Generic template for any other GCP service
   - Includes pricing factors, tips, and suggested questions

2. **list_services**: Queries the Cloud Billing API for all available services
   - Returns service IDs needed to query SKUs

3. **list_skus**: Lists SKUs for a specific service
   - Filterable by region and category
   - Returns SKU IDs needed for pricing queries

4. **get_sku_price**: Gets detailed pricing for a specific SKU
   - Supports multiple currencies (USD, JPY, EUR, etc.)
   - Returns tiered pricing information

5. **estimate_cost**: Calculates the final cost estimate
   - Takes SKU ID, usage amount, and context (service name, region, description)
   - Handles tiered pricing calculations

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

---

## Development

### Release Process

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions.

**To create a new release:**

```bash
# 1. Create and push a tag
git tag v0.6.0
git push origin v0.6.0
```

This will automatically:
1. Build binaries for all platforms (darwin/linux/windows, amd64/arm64)
2. Create a GitHub Release with changelog
3. Update the [homebrew-tap](https://github.com/nozomi-koborinai/homebrew-tap) Formula

**Prerequisites for homebrew-tap automation:**
- A GitHub Personal Access Token (PAT) with `repo` scope
- Store it as `HOMEBREW_TAP_TOKEN` in repository secrets

### Local Development

```bash
# Build
go build -o gcp-cost-mcp-server .

# Run locally
./gcp-cost-mcp-server

# Test GoReleaser config (dry run)
goreleaser release --snapshot --clean
```

## License

MIT License
