// Package freetier provides functionality for retrieving free tier information
// from GCP documentation when it's not available in the Cloud Billing Catalog API.
package freetier

import (
	"regexp"
	"strconv"
	"strings"
)

// FreeTierPattern represents a pattern for extracting free tier information
type FreeTierPattern struct {
	Regex    *regexp.Regexp
	Resource string
	Unit     string
}

// freeTierPatterns contains regex patterns to extract free tier information from documentation
var freeTierPatterns = []FreeTierPattern{
	// vCPU and memory time patterns (Cloud Run, Cloud Functions)
	{
		Regex:    regexp.MustCompile(`(?i)([0-9,]+)\s*vCPU[- ]?seconds?\s*(?:per\s*month|/month|monthly)?\s*(?:free|at no charge)`),
		Resource: "vCPU-seconds",
		Unit:     "seconds",
	},
	{
		Regex:    regexp.MustCompile(`(?i)([0-9,]+)\s*GiB[- ]?seconds?\s*(?:per\s*month|/month|monthly)?\s*(?:free|at no charge)`),
		Resource: "GiB-seconds",
		Unit:     "seconds",
	},
	// Alternative patterns with "free tier" prefix
	{
		Regex:    regexp.MustCompile(`(?i)free\s*tier[:\s]*([0-9,]+)\s*vCPU[- ]?seconds?`),
		Resource: "vCPU-seconds",
		Unit:     "seconds",
	},
	{
		Regex:    regexp.MustCompile(`(?i)free\s*tier[:\s]*([0-9,]+)\s*GiB[- ]?seconds?`),
		Resource: "GiB-seconds",
		Unit:     "seconds",
	},

	// Storage patterns (Cloud Storage, Firestore, etc.)
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9.]+)\s*(?:GB|GiB)\s*(?:of\s*storage\s*)?(?:per\s*month|/month|monthly)?\s*(?:is\s*)?free`),
		Resource: "storage",
		Unit:     "GiB",
	},
	{
		Regex:    regexp.MustCompile(`(?i)([0-9.]+)\s*(?:GB|GiB)\s*(?:of\s*)?(?:storage|data)\s*(?:per\s*month|/month|monthly)?\s*(?:is\s*)?free`),
		Resource: "storage",
		Unit:     "GiB",
	},

	// Request-based patterns (Cloud Functions, API Gateway, etc.)
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9.]+)\s*million\s*(?:invocations?|requests?)\s*(?:per\s*month|/month|monthly)?\s*(?:are\s*|is\s*)?free`),
		Resource: "requests",
		Unit:     "million",
	},
	{
		Regex:    regexp.MustCompile(`(?i)([0-9,]+)\s*(?:invocations?|requests?)\s*(?:per\s*month|/month|monthly)?\s*(?:are\s*|is\s*)?free`),
		Resource: "requests",
		Unit:     "count",
	},

	// Operation-based patterns (Firestore, Secret Manager, etc.)
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9,]+)\s*(?:document\s*)?(?:reads?|read\s*operations?)\s*(?:per\s*day|/day|daily)?\s*(?:are\s*|is\s*)?free`),
		Resource: "document-reads",
		Unit:     "count",
	},
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9,]+)\s*(?:document\s*)?(?:writes?|write\s*operations?)\s*(?:per\s*day|/day|daily)?\s*(?:are\s*|is\s*)?free`),
		Resource: "document-writes",
		Unit:     "count",
	},
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9,]+)\s*(?:document\s*)?(?:deletes?|delete\s*operations?)\s*(?:per\s*day|/day|daily)?\s*(?:are\s*|is\s*)?free`),
		Resource: "document-deletes",
		Unit:     "count",
	},

	// Secret Manager patterns
	{
		Regex:    regexp.MustCompile(`(?i)first\s*(\d+)\s*active\s*(?:secret\s*)?versions?\s*(?:are\s*|is\s*)?free`),
		Resource: "secret-versions",
		Unit:     "count",
	},
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9,]+)\s*access\s*operations?\s*(?:per\s*month|/month|monthly)?\s*(?:are\s*|is\s*)?free`),
		Resource: "access-operations",
		Unit:     "count",
	},

	// BigQuery patterns
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9.]+)\s*(?:TB|TiB)\s*(?:of\s*)?(?:query|queries|processing)\s*(?:per\s*month|/month|monthly)?\s*(?:is\s*)?free`),
		Resource: "query-processing",
		Unit:     "TiB",
	},

	// Network egress patterns
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9.]+)\s*(?:GB|GiB)\s*(?:of\s*)?(?:egress|outbound|network)\s*(?:per\s*month|/month|monthly)?\s*(?:is\s*)?free`),
		Resource: "egress",
		Unit:     "GiB",
	},

	// GKE patterns
	{
		Regex:    regexp.MustCompile(`(?i)\$([0-9.]+)/month\s*(?:credit|free)`),
		Resource: "cluster-credit",
		Unit:     "USD",
	},

	// Pub/Sub patterns
	{
		Regex:    regexp.MustCompile(`(?i)first\s*([0-9.]+)\s*(?:GB|GiB)\s*(?:of\s*)?(?:message|messaging)\s*(?:per\s*month|/month|monthly)?\s*(?:is\s*)?free`),
		Resource: "message-delivery",
		Unit:     "GiB",
	},
}

// scopePatterns helps identify if free tier applies per account or per project
var scopePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)per\s*billing\s*account`),
	regexp.MustCompile(`(?i)per\s*account`),
	regexp.MustCompile(`(?i)per\s*project`),
	regexp.MustCompile(`(?i)across\s*all\s*projects`),
}

// periodPatterns helps identify the free tier period
var periodPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)per\s*month|monthly|/month`),
	regexp.MustCompile(`(?i)per\s*day|daily|/day`),
	regexp.MustCompile(`(?i)always\s*free`),
}

// ExtractFreeTierItems extracts free tier information from documentation content
func ExtractFreeTierItems(content string) []FreeTierItem {
	var items []FreeTierItem
	seen := make(map[string]bool)

	for _, pattern := range freeTierPatterns {
		matches := pattern.Regex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			// Parse the amount
			amountStr := strings.ReplaceAll(match[1], ",", "")
			amount, err := strconv.ParseFloat(amountStr, 64)
			if err != nil {
				continue
			}

			// Convert million to actual count if needed
			if pattern.Unit == "million" {
				amount *= 1_000_000
			}

			// Avoid duplicates
			key := pattern.Resource + "-" + amountStr
			if seen[key] {
				continue
			}
			seen[key] = true

			unit := pattern.Unit
			if unit == "million" {
				unit = "count"
			}

			items = append(items, FreeTierItem{
				Resource: pattern.Resource,
				Amount:   amount,
				Unit:     unit,
			})
		}
	}

	return items
}

// ExtractScope determines if free tier applies per account or per project
func ExtractScope(content string) string {
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "per billing account") ||
		strings.Contains(contentLower, "per account") ||
		strings.Contains(contentLower, "across all projects") {
		return "account"
	}

	if strings.Contains(contentLower, "per project") {
		return "project"
	}

	// Default to account as most GCP free tiers are per billing account
	return "account"
}

// ExtractPeriod determines the free tier period (month, day, always)
func ExtractPeriod(content string) string {
	contentLower := strings.ToLower(content)

	if strings.Contains(contentLower, "per day") ||
		strings.Contains(contentLower, "daily") ||
		strings.Contains(contentLower, "/day") {
		return "day"
	}

	if strings.Contains(contentLower, "always free") {
		return "always"
	}

	// Default to month as most GCP free tiers are monthly
	return "month"
}
