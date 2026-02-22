package freetier

import (
	"testing"
)

func TestExtractFreeTierItems(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []FreeTierItem
	}{
		{
			name:    "Cloud Run vCPU-seconds",
			content: "The free tier includes 240,000 vCPU-seconds per month free of charge.",
			expected: []FreeTierItem{
				{Resource: "vCPU-seconds", Amount: 240000, Unit: "seconds"},
			},
		},
		{
			name:    "Cloud Run GiB-seconds",
			content: "450,000 GiB-seconds per month free",
			expected: []FreeTierItem{
				{Resource: "GiB-seconds", Amount: 450000, Unit: "seconds"},
			},
		},
		{
			name:    "Cloud Storage free tier",
			content: "First 5 GB of storage per month is free.",
			expected: []FreeTierItem{
				{Resource: "storage", Amount: 5, Unit: "GiB"},
			},
		},
		{
			name:    "Cloud Functions requests",
			content: "First 2 million invocations per month are free.",
			expected: []FreeTierItem{
				{Resource: "requests", Amount: 2000000, Unit: "count"},
			},
		},
		{
			name:    "Firestore document reads",
			content: "First 50,000 document reads per day are free.",
			expected: []FreeTierItem{
				{Resource: "document-reads", Amount: 50000, Unit: "count"},
			},
		},
		{
			name:    "Firestore document writes",
			content: "First 20,000 document writes per day are free.",
			expected: []FreeTierItem{
				{Resource: "document-writes", Amount: 20000, Unit: "count"},
			},
		},
		{
			name:    "Secret Manager versions",
			content: "First 6 active secret versions are free.",
			expected: []FreeTierItem{
				{Resource: "secret-versions", Amount: 6, Unit: "count"},
			},
		},
		{
			name:    "Secret Manager access operations",
			content: "First 10,000 access operations per month are free.",
			expected: []FreeTierItem{
				{Resource: "access-operations", Amount: 10000, Unit: "count"},
			},
		},
		{
			name:    "BigQuery query processing",
			content: "First 1 TB of queries per month is free.",
			expected: []FreeTierItem{
				{Resource: "query-processing", Amount: 1, Unit: "TiB"},
			},
		},
		{
			name:    "Network egress",
			content: "First 1 GB of egress per month is free.",
			expected: []FreeTierItem{
				{Resource: "egress", Amount: 1, Unit: "GiB"},
			},
		},
		{
			name:    "Pub/Sub message delivery",
			content: "First 10 GB of messaging per month is free.",
			expected: []FreeTierItem{
				{Resource: "message-delivery", Amount: 10, Unit: "GiB"},
			},
		},
		{
			name:     "No free tier mentioned",
			content:  "Cloud Spanner pricing is based on compute capacity and storage.",
			expected: nil,
		},
		{
			name:     "Empty content",
			content:  "",
			expected: nil,
		},
		{
			name: "Multiple free tier items",
			content: `Cloud Run pricing includes:
				- 240,000 vCPU-seconds per month free
				- 450,000 GiB-seconds per month free
				- First 2 million requests per month are free`,
			expected: []FreeTierItem{
				{Resource: "vCPU-seconds", Amount: 240000, Unit: "seconds"},
				{Resource: "GiB-seconds", Amount: 450000, Unit: "seconds"},
				{Resource: "requests", Amount: 2000000, Unit: "count"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFreeTierItems(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				t.Logf("Result: %+v", result)
				return
			}

			for i, expected := range tt.expected {
				if result[i].Resource != expected.Resource {
					t.Errorf("Item %d: expected resource %q, got %q", i, expected.Resource, result[i].Resource)
				}
				if result[i].Amount != expected.Amount {
					t.Errorf("Item %d: expected amount %f, got %f", i, expected.Amount, result[i].Amount)
				}
				if result[i].Unit != expected.Unit {
					t.Errorf("Item %d: expected unit %q, got %q", i, expected.Unit, result[i].Unit)
				}
			}
		})
	}
}

func TestExtractScope(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Per billing account",
			content:  "Free tier applies per billing account.",
			expected: "account",
		},
		{
			name:     "Per account",
			content:  "The free tier is per account.",
			expected: "account",
		},
		{
			name:     "Across all projects",
			content:  "Free usage is shared across all projects in your account.",
			expected: "account",
		},
		{
			name:     "Per project",
			content:  "Each project gets its own free tier allocation per project.",
			expected: "project",
		},
		{
			name:     "No scope mentioned - defaults to account",
			content:  "240,000 vCPU-seconds free per month.",
			expected: "account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractScope(tt.content)
			if result != tt.expected {
				t.Errorf("Expected scope %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractPeriod(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Per month",
			content:  "240,000 vCPU-seconds per month are free.",
			expected: "month",
		},
		{
			name:     "Monthly",
			content:  "Monthly free tier includes 2 million requests.",
			expected: "month",
		},
		{
			name:     "Per day",
			content:  "50,000 reads per day are free.",
			expected: "day",
		},
		{
			name:     "Daily",
			content:  "Daily free tier: 20,000 writes.",
			expected: "day",
		},
		{
			name:     "Always free",
			content:  "This service is always free for small workloads.",
			expected: "always",
		},
		{
			name:     "No period mentioned - defaults to month",
			content:  "First 5 GB of storage is free.",
			expected: "month",
		},
		{
			name: "Month dominant with some daily mentions (Cloud Run regression)",
			content: `Cloud Run pricing:
				180,000 vCPU-seconds per month free.
				360,000 GiB-seconds per month free.
				2 million requests per month free.
				Daily usage is metered and billed monthly.`,
			expected: "month",
		},
		{
			name: "Equal month and day mentions defaults to month",
			content: "Free tier: 1000 reads per day and 10 GB storage per month.",
			expected: "month",
		},
		{
			name:     "Day dominant",
			content:  "50,000 reads per day and 20,000 writes per day are free. Billed daily.",
			expected: "day",
		},
		{
			name:     "Slash notation month",
			content:  "Free tier: 1 TB/month of query processing.",
			expected: "month",
		},
		{
			name:     "Slash notation day",
			content:  "10,000 operations/day free.",
			expected: "day",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPeriod(tt.content)
			if result != tt.expected {
				t.Errorf("Expected period %q, got %q", tt.expected, result)
			}
		})
	}
}
