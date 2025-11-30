package tools

import (
	"fmt"
	"log"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// GetEstimationGuideInput is the input for the get_estimation_guide tool
type GetEstimationGuideInput struct {
	ServiceName string `json:"service_name" jsonschema_description:"The Google Cloud service name to get estimation requirements for. Works with ANY GCP service - common services get detailed guides, others get a comprehensive generic template. Examples: 'Cloud Run', 'BigQuery', 'Vertex AI', 'Cloud Logging', 'Dataflow', etc."`
}

// RequiredParameter represents a parameter needed for cost estimation
type RequiredParameter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Examples    []string `json:"examples,omitempty"`
	DefaultTip  string   `json:"default_tip,omitempty"`
}

// EstimationGuide represents the guide for estimating costs
type EstimationGuide struct {
	ServiceName        string              `json:"service_name"`
	ServiceDescription string              `json:"service_description"`
	Parameters         []RequiredParameter `json:"parameters"`
	PricingFactors     []string            `json:"pricing_factors"`
	Tips               []string            `json:"tips,omitempty"`
	RelatedServices    []string            `json:"related_services,omitempty"`
}

// GetEstimationGuideOutput is the output of the get_estimation_guide tool
type GetEstimationGuideOutput struct {
	Guide             EstimationGuide `json:"guide"`
	SuggestedQuestion string          `json:"suggested_question"`
}

// serviceGuides contains estimation guides for various GCP services
var serviceGuides = map[string]EstimationGuide{
	"cloud run": {
		ServiceName:        "Cloud Run",
		ServiceDescription: "Fully managed serverless platform for containerized applications",
		Parameters: []RequiredParameter{
			{
				Name:        "billing_type",
				Description: "Billing model: instance-based (always allocated) or request-based (pay per request)",
				Required:    true,
				Examples:    []string{"instance-based", "request-based"},
				DefaultTip:  "Instance-based for consistent traffic, request-based for sporadic traffic",
			},
			{
				Name:        "region",
				Description: "The region where the service will be deployed",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
				DefaultTip:  "Choose a region close to your users for lower latency. Prices vary by region.",
			},
			{
				Name:        "vcpu",
				Description: "Number of vCPUs allocated per instance",
				Required:    true,
				Examples:    []string{"1", "2", "4", "8"},
				DefaultTip:  "Start with 1 vCPU for most workloads",
			},
			{
				Name:        "memory_gib",
				Description: "Memory allocated per instance in GiB",
				Required:    true,
				Examples:    []string{"0.5", "1", "2", "4", "8", "16", "32"},
				DefaultTip:  "Minimum 512MiB, typically 1-2GiB for web applications",
			},
			{
				Name:        "instance_count",
				Description: "Number of instances (min instances for always-on, or expected concurrent instances)",
				Required:    true,
				Examples:    []string{"0", "1", "2", "5", "10"},
				DefaultTip:  "Set min instances to 0 for cost savings if cold starts are acceptable",
			},
			{
				Name:        "monthly_seconds",
				Description: "Expected billable seconds per month per instance (or total active seconds for request-based)",
				Required:    true,
				Examples:    []string{"2628000 (730h*3600s)", "633600 (176h*3600s)"},
				DefaultTip:  "2,628,000 seconds = full month (730 hours). For request-based, calculate based on request duration.",
			},
			{
				Name:        "requests_per_month",
				Description: "Expected number of requests per month (for request-based billing)",
				Required:    false,
				Examples:    []string{"1000000", "10000000", "100000000"},
				DefaultTip:  "First 2 million requests/month are free (request-based billing only)",
			},
			{
				Name:        "concurrency",
				Description: "Max concurrent requests per instance",
				Required:    false,
				Examples:    []string{"1", "80", "250", "1000"},
				DefaultTip:  "Higher concurrency = fewer instances needed = lower cost",
			},
			{
				Name:        "gpu_type",
				Description: "GPU type if needed (optional)",
				Required:    false,
				Examples:    []string{"none", "NVIDIA-L4"},
				DefaultTip:  "GPUs are charged per second. L4 GPU: ~$0.00019/sec without zonal redundancy",
			},
			{
				Name:        "egress_gib",
				Description: "Expected outbound data transfer in GiB per month",
				Required:    false,
				Examples:    []string{"1", "10", "100", "1000"},
				DefaultTip:  "Ingress is free. First 1 GiB egress to North America is free.",
			},
		},
		PricingFactors: []string{
			"vCPU time (per vCPU-second): ~$0.000024 active, ~$0.0000025 idle (us-central1)",
			"Memory time (per GiB-second): ~$0.0000025 (us-central1)",
			"Number of requests (request-based only): $0.40 per million after free tier",
			"GPU time (if used): ~$0.00019-0.00029/sec for L4",
			"Networking egress",
		},
		Tips: []string{
			"Free tier (instance-based): 240,000 vCPU-seconds, 450,000 GiB-seconds per month",
			"Free tier (request-based): 180,000 vCPU-seconds, 360,000 GiB-seconds, 2M requests per month",
			"Committed use discounts (CUDs): 1-year saves ~17%, 3-year saves ~17% (Cloud Run CUD) or up to 46% (Compute Flexible CUD)",
			"Use CPU allocation only during request processing to reduce costs (request-based)",
			"Consider startup CPU boost for faster cold starts without increasing base cost",
		},
		RelatedServices: []string{"Cloud Load Balancing", "Cloud SQL", "Secret Manager", "VPC Connector", "Eventarc"},
	},
	"compute engine": {
		ServiceName:        "Compute Engine",
		ServiceDescription: "Virtual machines running on Google's infrastructure",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "The region/zone where the VM will be deployed",
				Required:    true,
				Examples:    []string{"asia-northeast1-a (Tokyo)", "us-central1-a", "europe-west1-b"},
			},
			{
				Name:        "machine_type",
				Description: "The machine type (vCPU and memory combination)",
				Required:    true,
				Examples:    []string{"e2-micro", "e2-medium", "n2-standard-2", "n2-standard-4", "c2-standard-4"},
				DefaultTip:  "E2 series is cost-effective for general workloads",
			},
			{
				Name:        "instance_count",
				Description: "Number of VM instances",
				Required:    true,
				Examples:    []string{"1", "2", "5", "10"},
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    true,
				Examples:    []string{"730 (24/7)", "176 (business hours)", "100"},
			},
			{
				Name:        "disk_type",
				Description: "Boot disk type",
				Required:    true,
				Examples:    []string{"pd-standard (HDD)", "pd-balanced", "pd-ssd"},
				DefaultTip:  "pd-balanced offers good price/performance ratio",
			},
			{
				Name:        "disk_size_gb",
				Description: "Boot disk size in GB",
				Required:    true,
				Examples:    []string{"10", "50", "100", "500"},
				DefaultTip:  "Minimum 10GB for most OS images",
			},
			{
				Name:        "preemptible",
				Description: "Use preemptible/spot VMs for up to 91% discount",
				Required:    false,
				Examples:    []string{"true", "false"},
				DefaultTip:  "Good for fault-tolerant batch workloads",
			},
			{
				Name:        "egress_gb",
				Description: "Expected outbound data transfer in GB per month",
				Required:    false,
				Examples:    []string{"10", "100", "1000"},
			},
		},
		PricingFactors: []string{
			"vCPU hours",
			"Memory hours",
			"Persistent disk storage",
			"Networking egress",
			"OS licensing (for premium OS)",
		},
		Tips: []string{
			"Committed use discounts (CUDs) can save up to 57% for 1-year or 70% for 3-year commitments",
			"Sustained use discounts (SUDs) automatically apply for running VMs over 25% of the month",
			"Preemptible VMs offer up to 91% discount but can be terminated with 30s notice",
		},
		RelatedServices: []string{"Persistent Disk", "Cloud Load Balancing", "Cloud NAT", "VPC"},
	},
	"cloud storage": {
		ServiceName:        "Cloud Storage",
		ServiceDescription: "Object storage for any amount of data",
		Parameters: []RequiredParameter{
			{
				Name:        "location",
				Description: "Storage location type and region",
				Required:    true,
				Examples:    []string{"asia-northeast1 (regional)", "asia (multi-region)", "us (multi-region)"},
				DefaultTip:  "Regional is cheaper, multi-region provides higher availability",
			},
			{
				Name:        "storage_class",
				Description: "Storage class based on access frequency",
				Required:    true,
				Examples:    []string{"STANDARD", "NEARLINE", "COLDLINE", "ARCHIVE"},
				DefaultTip:  "STANDARD for frequently accessed, ARCHIVE for rarely accessed data",
			},
			{
				Name:        "storage_gb",
				Description: "Total data stored in GB",
				Required:    true,
				Examples:    []string{"100", "1000", "10000"},
			},
			{
				Name:        "class_a_operations",
				Description: "Monthly Class A operations (create, list) in thousands",
				Required:    false,
				Examples:    []string{"10", "100", "1000"},
				DefaultTip:  "Writes, lists are Class A (more expensive)",
			},
			{
				Name:        "class_b_operations",
				Description: "Monthly Class B operations (read, get) in thousands",
				Required:    false,
				Examples:    []string{"100", "1000", "10000"},
				DefaultTip:  "Reads are Class B (cheaper)",
			},
			{
				Name:        "egress_gb",
				Description: "Monthly data retrieved/transferred out in GB",
				Required:    false,
				Examples:    []string{"10", "100", "1000"},
			},
		},
		PricingFactors: []string{
			"Data storage (per GB/month)",
			"Network egress",
			"Operations (Class A and B)",
			"Early deletion fee (for Nearline/Coldline/Archive)",
			"Retrieval fee (for Nearline/Coldline/Archive)",
		},
		Tips: []string{
			"Use Object Lifecycle Management to automatically transition data to cheaper storage classes",
			"Nearline has 30-day minimum, Coldline 90-day, Archive 365-day minimum storage duration",
			"Consider using Autoclass for automatic storage class management",
		},
		RelatedServices: []string{"Cloud CDN", "Transfer Service", "BigQuery"},
	},
	"bigquery": {
		ServiceName:        "BigQuery",
		ServiceDescription: "Serverless, highly scalable data warehouse",
		Parameters: []RequiredParameter{
			{
				Name:        "pricing_model",
				Description: "Pricing model for compute",
				Required:    true,
				Examples:    []string{"on-demand", "capacity (slots)"},
				DefaultTip:  "On-demand is pay-per-query, capacity is flat-rate with reserved slots",
			},
			{
				Name:        "storage_gb",
				Description: "Total active storage in GB",
				Required:    true,
				Examples:    []string{"100", "1000", "10000"},
				DefaultTip:  "First 10GB/month is free",
			},
			{
				Name:        "query_tb_per_month",
				Description: "Expected query processing in TB per month (for on-demand)",
				Required:    false,
				Examples:    []string{"1", "10", "100"},
				DefaultTip:  "First 1TB/month is free for on-demand",
			},
			{
				Name:        "slots",
				Description: "Number of slots for capacity pricing",
				Required:    false,
				Examples:    []string{"100", "500", "2000"},
			},
			{
				Name:        "streaming_inserts_gb",
				Description: "Monthly streaming insert volume in GB",
				Required:    false,
				Examples:    []string{"10", "100", "1000"},
			},
			{
				Name:        "location",
				Description: "Dataset location",
				Required:    true,
				Examples:    []string{"US (multi-region)", "asia-northeast1", "EU"},
			},
		},
		PricingFactors: []string{
			"Query processing (on-demand: per TB scanned)",
			"Slot hours (capacity pricing)",
			"Active storage (per GB/month)",
			"Long-term storage (50% cheaper after 90 days)",
			"Streaming inserts",
		},
		Tips: []string{
			"Use partitioning and clustering to reduce query costs",
			"Long-term storage (data not modified for 90 days) is 50% cheaper",
			"Consider BigQuery Editions for more predictable pricing",
			"Use LIMIT clause and column selection to reduce scanned data",
		},
		RelatedServices: []string{"Cloud Storage", "Dataflow", "Looker", "Data Studio"},
	},
	"cloud sql": {
		ServiceName:        "Cloud SQL",
		ServiceDescription: "Fully managed relational database service for MySQL, PostgreSQL, and SQL Server",
		Parameters: []RequiredParameter{
			{
				Name:        "database_type",
				Description: "Database engine type",
				Required:    true,
				Examples:    []string{"MySQL", "PostgreSQL", "SQL Server"},
			},
			{
				Name:        "region",
				Description: "The region where the instance will be deployed",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
			},
			{
				Name:        "machine_type",
				Description: "vCPU and memory configuration",
				Required:    true,
				Examples:    []string{"db-f1-micro", "db-g1-small", "db-custom-2-4096", "db-n1-standard-2"},
				DefaultTip:  "db-f1-micro is good for development, use db-custom for production",
			},
			{
				Name:        "instance_count",
				Description: "Number of Cloud SQL instances",
				Required:    true,
				Examples:    []string{"1", "2"},
			},
			{
				Name:        "storage_type",
				Description: "Storage type",
				Required:    true,
				Examples:    []string{"SSD", "HDD"},
				DefaultTip:  "SSD recommended for production workloads",
			},
			{
				Name:        "storage_gb",
				Description: "Storage capacity in GB",
				Required:    true,
				Examples:    []string{"10", "100", "500", "1000"},
			},
			{
				Name:        "high_availability",
				Description: "Enable high availability (regional)",
				Required:    true,
				Examples:    []string{"true", "false"},
				DefaultTip:  "HA doubles the cost but provides failover capability",
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    false,
				Examples:    []string{"730 (24/7)", "176 (business hours)"},
				DefaultTip:  "Cloud SQL instances typically run 24/7",
			},
			{
				Name:        "backup_gb",
				Description: "Backup storage in GB",
				Required:    false,
				Examples:    []string{"10", "100"},
			},
		},
		PricingFactors: []string{
			"vCPU hours",
			"Memory hours",
			"Storage (SSD/HDD per GB/month)",
			"Backup storage",
			"Network egress",
			"HA configuration (doubles compute cost)",
		},
		Tips: []string{
			"Use shared-core instances (f1-micro, g1-small) for development to save costs",
			"Consider read replicas instead of HA if you need read scaling",
			"Enable automatic storage increase to avoid running out of space",
			"SQL Server requires additional licensing costs",
		},
		RelatedServices: []string{"Cloud Run", "Compute Engine", "App Engine", "VPC"},
	},
	"gke": {
		ServiceName:        "Google Kubernetes Engine (GKE)",
		ServiceDescription: "Managed Kubernetes service for containerized applications",
		Parameters: []RequiredParameter{
			{
				Name:        "mode",
				Description: "GKE operation mode",
				Required:    true,
				Examples:    []string{"Autopilot", "Standard"},
				DefaultTip:  "Autopilot: Google manages nodes, pay per pod. Standard: You manage nodes, pay per node.",
			},
			{
				Name:        "region",
				Description: "Cluster region or zone",
				Required:    true,
				Examples:    []string{"asia-northeast1 (regional)", "asia-northeast1-a (zonal)", "us-central1"},
				DefaultTip:  "Regional clusters provide higher availability but cost more",
			},
			{
				Name:        "cluster_type",
				Description: "Cluster topology",
				Required:    true,
				Examples:    []string{"zonal", "regional"},
				DefaultTip:  "Zonal: $74.40/month free tier. Regional: No free tier, higher availability",
			},
			{
				Name:        "node_count",
				Description: "Number of nodes (for Standard mode)",
				Required:    false,
				Examples:    []string{"3", "5", "10", "20"},
				DefaultTip:  "Minimum 1 node per zone. Regional clusters have nodes in multiple zones.",
			},
			{
				Name:        "machine_type",
				Description: "Node machine type (for Standard mode)",
				Required:    false,
				Examples:    []string{"e2-medium", "e2-standard-4", "n2-standard-2", "n2-standard-8"},
				DefaultTip:  "E2 series is cost-effective for general workloads",
			},
			{
				Name:        "pod_vcpu",
				Description: "Total vCPU requested by pods (for Autopilot mode)",
				Required:    false,
				Examples:    []string{"4", "8", "16", "32"},
				DefaultTip:  "Autopilot charges based on pod resource requests, not node capacity",
			},
			{
				Name:        "pod_memory_gib",
				Description: "Total memory requested by pods in GiB (for Autopilot mode)",
				Required:    false,
				Examples:    []string{"8", "16", "32", "64"},
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    true,
				Examples:    []string{"730 (24/7)", "176 (business hours)"},
				DefaultTip:  "Clusters typically run 24/7. Cluster management fee: $0.10/hour",
			},
			{
				Name:        "spot_nodes",
				Description: "Use Spot VMs for nodes (Standard) or Spot pods (Autopilot)",
				Required:    false,
				Examples:    []string{"true", "false"},
				DefaultTip:  "Spot can save up to 91% but may be preempted",
			},
		},
		PricingFactors: []string{
			"Cluster management fee: $0.10/hour per cluster (zonal gets $74.40/month free credit)",
			"Autopilot: vCPU (~$0.0445/vCPU-hour), Memory (~$0.0049/GiB-hour), Ephemeral storage",
			"Standard: Compute Engine pricing for nodes",
			"Persistent disk storage",
			"Networking egress",
		},
		Tips: []string{
			"Free tier: $74.40/month credit for one Autopilot or zonal Standard cluster",
			"Autopilot includes security features, automatic upgrades, and right-sizing",
			"Use Spot VMs/pods for fault-tolerant workloads to save up to 91%",
			"Committed use discounts apply to both Standard nodes and Autopilot pods",
			"Consider GKE Autopilot for simpler management and optimized costs",
		},
		RelatedServices: []string{"Compute Engine", "Cloud Load Balancing", "Artifact Registry", "Cloud Logging"},
	},
	"cloud functions": {
		ServiceName:        "Cloud Functions (1st gen)",
		ServiceDescription: "Event-driven serverless functions (1st generation)",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "Function deployment region",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
				DefaultTip:  "Choose region close to event sources and downstream services",
			},
			{
				Name:        "memory_mb",
				Description: "Memory allocated to function in MB",
				Required:    true,
				Examples:    []string{"128", "256", "512", "1024", "2048", "4096", "8192"},
				DefaultTip:  "More memory = more vCPU allocated proportionally",
			},
			{
				Name:        "invocations_per_month",
				Description: "Expected number of function invocations per month",
				Required:    true,
				Examples:    []string{"1000000", "10000000", "100000000"},
				DefaultTip:  "First 2 million invocations/month are free",
			},
			{
				Name:        "avg_execution_time_ms",
				Description: "Average execution time per invocation in milliseconds",
				Required:    true,
				Examples:    []string{"100", "500", "1000", "5000"},
				DefaultTip:  "Billed in 100ms increments, minimum 100ms",
			},
			{
				Name:        "egress_gib",
				Description: "Expected outbound data transfer in GiB per month",
				Required:    false,
				Examples:    []string{"1", "5", "10"},
				DefaultTip:  "First 5 GiB egress/month is free",
			},
		},
		PricingFactors: []string{
			"Invocations: $0.40 per million (first 2M free)",
			"Compute time: varies by memory tier (e.g., 256MB: $0.000000463/100ms)",
			"Networking egress: first 5 GiB free, then standard rates",
		},
		Tips: []string{
			"Free tier: 2M invocations, 400,000 GB-seconds, 200,000 GHz-seconds, 5 GiB egress per month",
			"Consider Cloud Run functions (2nd gen) for more flexibility and longer timeouts",
			"Memory selection affects allocated vCPU proportionally",
			"Use minimum memory needed to reduce costs",
		},
		RelatedServices: []string{"Cloud Pub/Sub", "Cloud Storage", "Firestore", "Cloud Scheduler"},
	},
	"cloud run functions": {
		ServiceName:        "Cloud Run functions (2nd gen)",
		ServiceDescription: "Event-driven serverless functions built on Cloud Run",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "Function deployment region",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
			},
			{
				Name:        "vcpu",
				Description: "Number of vCPUs allocated",
				Required:    true,
				Examples:    []string{"1", "2", "4"},
				DefaultTip:  "Decoupled from memory, choose independently",
			},
			{
				Name:        "memory_gib",
				Description: "Memory allocated in GiB",
				Required:    true,
				Examples:    []string{"0.5", "1", "2", "4", "8", "16"},
			},
			{
				Name:        "invocations_per_month",
				Description: "Expected number of invocations per month",
				Required:    true,
				Examples:    []string{"1000000", "10000000"},
			},
			{
				Name:        "avg_execution_time_ms",
				Description: "Average execution time per invocation in milliseconds",
				Required:    true,
				Examples:    []string{"100", "500", "1000", "10000"},
				DefaultTip:  "2nd gen supports up to 60 minutes execution time",
			},
			{
				Name:        "concurrency",
				Description: "Concurrent requests per instance",
				Required:    false,
				Examples:    []string{"1", "80", "1000"},
				DefaultTip:  "Higher concurrency can reduce costs significantly",
			},
		},
		PricingFactors: []string{
			"Same as Cloud Run pricing (vCPU-seconds, GiB-seconds)",
			"Eventarc charges may apply for certain triggers",
		},
		Tips: []string{
			"Priced same as Cloud Run - see Cloud Run for detailed pricing",
			"Supports longer execution times (up to 60 min vs 9 min for 1st gen)",
			"Use concurrency > 1 to handle multiple requests per instance",
			"Consider Cloud Run directly for more control over scaling",
		},
		RelatedServices: []string{"Cloud Run", "Eventarc", "Cloud Pub/Sub", "Cloud Storage"},
	},
	"app engine standard": {
		ServiceName:        "App Engine Standard Environment",
		ServiceDescription: "Fully managed serverless application platform with automatic scaling",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "App Engine region (cannot be changed after creation)",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central", "europe-west"},
				DefaultTip:  "Choose carefully - region cannot be changed for existing projects",
			},
			{
				Name:        "instance_class",
				Description: "Instance class determining CPU and memory",
				Required:    true,
				Examples:    []string{"F1 (256MB)", "F2 (512MB)", "F4 (1GB)", "F4_1G (1GB+)"},
				DefaultTip:  "F1 is included in free tier (28 instance-hours/day)",
			},
			{
				Name:        "instance_hours_per_day",
				Description: "Expected instance hours per day",
				Required:    true,
				Examples:    []string{"24", "28 (free tier)", "100", "720"},
				DefaultTip:  "Free tier: 28 F1 instance-hours or 9 B1 instance-hours per day",
			},
			{
				Name:        "scaling_type",
				Description: "Scaling configuration",
				Required:    true,
				Examples:    []string{"automatic", "basic", "manual"},
				DefaultTip:  "Automatic scales based on traffic, basic scales on request queue",
			},
			{
				Name:        "egress_gib",
				Description: "Expected outbound data transfer in GiB per day",
				Required:    false,
				Examples:    []string{"1 (free)", "10", "100"},
				DefaultTip:  "First 1 GiB/day is free",
			},
		},
		PricingFactors: []string{
			"Instance hours by class (F1: $0.05/hour, F2: $0.10/hour, F4: $0.20/hour, F4_1G: $0.30/hour)",
			"Outbound bandwidth: $0.12/GiB after 1 GiB/day free",
			"Datastore/Firestore operations (if used)",
			"Cloud Storage (if used)",
		},
		Tips: []string{
			"Generous free tier: 28 F1 instance-hours/day, 1 GiB egress/day",
			"Idle instances accrue charges for 15 minutes after last request",
			"Use automatic scaling with appropriate min/max instances",
			"Consider Cloud Run for new projects - more flexible pricing",
		},
		RelatedServices: []string{"Cloud Datastore", "Cloud Tasks", "Cloud Scheduler", "Memcache"},
	},
	"app engine flexible": {
		ServiceName:        "App Engine Flexible Environment",
		ServiceDescription: "Managed application platform running on configurable Compute Engine VMs",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "App Engine region",
				Required:    true,
				Examples:    []string{"asia-northeast1", "us-central1", "europe-west1"},
			},
			{
				Name:        "vcpu",
				Description: "Number of vCPUs per instance",
				Required:    true,
				Examples:    []string{"1", "2", "4", "8"},
			},
			{
				Name:        "memory_gb",
				Description: "Memory per instance in GB",
				Required:    true,
				Examples:    []string{"0.9", "1.8", "3.6", "7.2"},
				DefaultTip:  "Memory is charged including runtime overhead",
			},
			{
				Name:        "instance_count",
				Description: "Number of instances",
				Required:    true,
				Examples:    []string{"1", "2", "5", "10"},
				DefaultTip:  "Minimum 1 instance always running",
			},
			{
				Name:        "disk_gb",
				Description: "Persistent disk size in GB per instance",
				Required:    true,
				Examples:    []string{"10", "50", "100"},
				DefaultTip:  "10 GB minimum",
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    true,
				Examples:    []string{"730 (24/7)"},
				DefaultTip:  "Flexible environment requires at least 1 instance always running",
			},
		},
		PricingFactors: []string{
			"vCPU hours: ~$0.0526/vCPU-hour",
			"Memory hours: ~$0.0071/GB-hour",
			"Persistent disk: ~$0.0400/GB-month",
			"No free tier for flexible environment",
		},
		Tips: []string{
			"No free tier - charges start from first instance",
			"Billed per second with 1-minute minimum",
			"Consider Cloud Run for lower costs with similar capabilities",
			"Useful for applications requiring custom runtimes or specific dependencies",
		},
		RelatedServices: []string{"Compute Engine", "Cloud SQL", "Cloud Storage"},
	},
	"pub/sub": {
		ServiceName:        "Cloud Pub/Sub",
		ServiceDescription: "Messaging and event ingestion service for streaming analytics and event-driven systems",
		Parameters: []RequiredParameter{
			{
				Name:        "throughput_tib_per_month",
				Description: "Expected message throughput in TiB per month",
				Required:    true,
				Examples:    []string{"0.01", "0.1", "1", "10"},
				DefaultTip:  "First 10 GiB/month is free",
			},
			{
				Name:        "subscription_count",
				Description: "Number of subscriptions per topic (affects delivery throughput)",
				Required:    true,
				Examples:    []string{"1", "2", "5"},
				DefaultTip:  "Each subscription delivers messages independently",
			},
			{
				Name:        "subscription_type",
				Description: "Type of subscription",
				Required:    true,
				Examples:    []string{"pull", "push", "BigQuery", "Cloud Storage"},
				DefaultTip:  "BigQuery/Cloud Storage subscriptions cost $50/TiB vs $40/TiB for pull/push",
			},
			{
				Name:        "message_retention_days",
				Description: "Message retention period in days",
				Required:    false,
				Examples:    []string{"1", "7", "31"},
				DefaultTip:  "Storage fees apply: $0.27/GiB-month for retained messages",
			},
			{
				Name:        "avg_message_size_kb",
				Description: "Average message size in KB",
				Required:    false,
				Examples:    []string{"0.1", "1", "10", "100"},
				DefaultTip:  "Minimum 1 KB charged per request regardless of actual size",
			},
		},
		PricingFactors: []string{
			"Message Delivery Basic: $40/TiB (first 10 GiB free)",
			"BigQuery subscription: $50/TiB",
			"Cloud Storage subscription: $50/TiB",
			"Import topics: $50-80/TiB depending on source",
			"Message storage: $0.27/GiB-month",
		},
		Tips: []string{
			"Free tier: 10 GiB/month for basic message delivery",
			"Batch messages to minimize per-request overhead (1 KB minimum)",
			"Consider Pub/Sub Lite for cost-sensitive, high-throughput workloads",
			"Message size includes body, attributes, timestamp, and message_id",
		},
		RelatedServices: []string{"Dataflow", "Cloud Functions", "BigQuery", "Cloud Storage"},
	},
	"firestore": {
		ServiceName:        "Firestore",
		ServiceDescription: "Flexible, scalable NoSQL document database for mobile, web, and server development",
		Parameters: []RequiredParameter{
			{
				Name:        "location",
				Description: "Database location",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "nam5 (multi-region)", "eur3 (multi-region)"},
				DefaultTip:  "Multi-region locations provide higher availability at higher cost",
			},
			{
				Name:        "document_reads_per_day",
				Description: "Expected document reads per day",
				Required:    true,
				Examples:    []string{"50000 (free)", "100000", "1000000", "10000000"},
				DefaultTip:  "First 50,000 reads/day are free",
			},
			{
				Name:        "document_writes_per_day",
				Description: "Expected document writes per day",
				Required:    true,
				Examples:    []string{"20000 (free)", "50000", "500000"},
				DefaultTip:  "First 20,000 writes/day are free",
			},
			{
				Name:        "document_deletes_per_day",
				Description: "Expected document deletes per day",
				Required:    true,
				Examples:    []string{"20000 (free)", "50000"},
				DefaultTip:  "First 20,000 deletes/day are free",
			},
			{
				Name:        "storage_gib",
				Description: "Expected stored data in GiB",
				Required:    true,
				Examples:    []string{"1 (free)", "10", "100", "1000"},
				DefaultTip:  "First 1 GiB storage is free",
			},
			{
				Name:        "egress_gib_per_month",
				Description: "Expected outbound data transfer in GiB per month",
				Required:    false,
				Examples:    []string{"10 (free)", "100"},
				DefaultTip:  "First 10 GiB/month egress is free",
			},
		},
		PricingFactors: []string{
			"Document reads: $0.03-0.06/100,000 (varies by region)",
			"Document writes: $0.09-0.18/100,000 (varies by region)",
			"Document deletes: $0.01-0.02/100,000 (varies by region)",
			"Stored data: ~$0.15-0.18/GiB-month (varies by region)",
			"Network egress",
		},
		Tips: []string{
			"Generous free tier: 50K reads, 20K writes, 20K deletes per day, 1 GiB storage",
			"Use batch operations to reduce individual operation counts",
			"Index reads are charged separately (1 read per 1000 index entries)",
			"Consider data modeling to minimize reads (denormalization)",
			"Committed use discounts available (1-year: 20%, 3-year: 40%)",
		},
		RelatedServices: []string{"Cloud Functions", "Firebase", "App Engine", "Cloud Run"},
	},
	"spanner": {
		ServiceName:        "Cloud Spanner",
		ServiceDescription: "Fully managed, scalable, relational database with unlimited scale and 99.999% availability",
		Parameters: []RequiredParameter{
			{
				Name:        "region_config",
				Description: "Regional or multi-region configuration",
				Required:    true,
				Examples:    []string{"regional (asia-northeast1)", "multi-region (nam3)", "multi-region (nam-eur-asia1)"},
				DefaultTip:  "Regional is ~3x cheaper than multi-region",
			},
			{
				Name:        "node_count",
				Description: "Number of nodes (or processing units / 1000)",
				Required:    true,
				Examples:    []string{"1", "3", "5", "10"},
				DefaultTip:  "1 node = 1000 processing units. Minimum 100 PU for small workloads.",
			},
			{
				Name:        "processing_units",
				Description: "Processing units for smaller workloads (100-900 PU)",
				Required:    false,
				Examples:    []string{"100", "500", "900"},
				DefaultTip:  "Use PU for workloads smaller than 1 node. 100 PU minimum.",
			},
			{
				Name:        "storage_gib",
				Description: "Expected stored data in GiB",
				Required:    true,
				Examples:    []string{"10", "100", "1000", "10000"},
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    true,
				Examples:    []string{"730 (24/7)"},
				DefaultTip:  "Spanner typically runs 24/7 for production workloads",
			},
		},
		PricingFactors: []string{
			"Node/hour: $0.90/node-hour (regional), $3.00/node-hour (multi-region)",
			"Processing unit/hour: $0.0009/PU-hour (regional)",
			"Storage: $0.30/GiB-month",
			"Network egress (cross-region)",
		},
		Tips: []string{
			"Start with processing units (100 PU minimum) for smaller workloads",
			"Regional instances are ~3x cheaper than multi-region",
			"Committed use discounts: 1-year (20%), 3-year (40%)",
			"Consider autoscaling to optimize costs based on load",
			"Spanner is best for high-scale, globally distributed workloads",
		},
		RelatedServices: []string{"BigQuery", "Dataflow", "Cloud SQL"},
	},
	"memorystore redis": {
		ServiceName:        "Memorystore for Redis",
		ServiceDescription: "Fully managed Redis service for caching and real-time analytics",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "Instance region",
				Required:    true,
				Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1"},
			},
			{
				Name:        "tier",
				Description: "Service tier",
				Required:    true,
				Examples:    []string{"Basic", "Standard (HA)"},
				DefaultTip:  "Standard tier provides automatic failover and replication",
			},
			{
				Name:        "capacity_gb",
				Description: "Instance memory capacity in GB",
				Required:    true,
				Examples:    []string{"1", "5", "10", "50", "100"},
				DefaultTip:  "Basic: 1-300 GB, Standard: 5-300 GB",
			},
			{
				Name:        "monthly_hours",
				Description: "Expected running hours per month",
				Required:    true,
				Examples:    []string{"730 (24/7)"},
				DefaultTip:  "Redis instances typically run 24/7",
			},
		},
		PricingFactors: []string{
			"Basic tier: ~$0.016-0.049/GB-hour (varies by region)",
			"Standard tier: ~$0.032-0.098/GB-hour (varies by region)",
			"Network egress (same region is free)",
		},
		Tips: []string{
			"No free tier - charges from first GB-hour",
			"Standard tier costs ~2x Basic but provides HA and replication",
			"Use Basic tier for caching workloads that can tolerate data loss",
			"Consider Redis Cluster mode for larger workloads (>300 GB)",
		},
		RelatedServices: []string{"Cloud Run", "Compute Engine", "GKE", "App Engine"},
	},
	"cloud cdn": {
		ServiceName:        "Cloud CDN",
		ServiceDescription: "Content delivery network for fast, reliable web and video content delivery",
		Parameters: []RequiredParameter{
			{
				Name:        "cache_egress_gib",
				Description: "Expected cache egress (cache hits) in GiB per month",
				Required:    true,
				Examples:    []string{"100", "1000", "10000", "100000"},
				DefaultTip:  "Cache hits are cheaper than cache misses",
			},
			{
				Name:        "cache_fill_gib",
				Description: "Expected cache fill (cache misses) in GiB per month",
				Required:    true,
				Examples:    []string{"10", "100", "1000"},
				DefaultTip:  "Minimize cache misses for cost efficiency",
			},
			{
				Name:        "http_requests_millions",
				Description: "Expected HTTP/HTTPS requests in millions per month",
				Required:    true,
				Examples:    []string{"1", "10", "100", "1000"},
			},
			{
				Name:        "cache_invalidation_requests",
				Description: "Expected cache invalidation requests per month",
				Required:    false,
				Examples:    []string{"0", "10", "100"},
				DefaultTip:  "First 10,000 invalidation paths/month are free, then $0.005/path",
			},
		},
		PricingFactors: []string{
			"Cache egress: $0.02-0.20/GiB (varies by destination region)",
			"Cache fill: ~$0.01/GiB",
			"HTTP requests: $0.0075/10,000 requests",
			"Cache invalidation: first 10K paths free, then $0.005/path",
		},
		Tips: []string{
			"Higher cache hit ratio = lower costs",
			"Use appropriate Cache-Control headers to maximize cache efficiency",
			"Consider Cloud CDN for static content and Media CDN for video streaming",
			"Combine with Cloud Load Balancing for optimal performance",
		},
		RelatedServices: []string{"Cloud Load Balancing", "Cloud Storage", "Compute Engine"},
	},
	"cloud armor": {
		ServiceName:        "Cloud Armor",
		ServiceDescription: "DDoS protection and web application firewall (WAF) service",
		Parameters: []RequiredParameter{
			{
				Name:        "tier",
				Description: "Cloud Armor tier",
				Required:    true,
				Examples:    []string{"Standard", "Plus (Managed Protection)"},
				DefaultTip:  "Standard for basic protection, Plus for advanced managed protection",
			},
			{
				Name:        "policies",
				Description: "Number of security policies",
				Required:    true,
				Examples:    []string{"1", "5", "10"},
			},
			{
				Name:        "rules_per_policy",
				Description: "Number of rules per policy",
				Required:    true,
				Examples:    []string{"5", "10", "50"},
				DefaultTip:  "First 5 rules per policy included, additional rules charged",
			},
			{
				Name:        "requests_millions",
				Description: "Expected requests evaluated per month in millions",
				Required:    true,
				Examples:    []string{"1", "10", "100", "1000"},
			},
		},
		PricingFactors: []string{
			"Policy: $5/policy/month",
			"Rules: First 5 rules/policy free, then $1/rule/month",
			"Requests: $0.75/million requests",
			"Plus tier: Additional subscription fee for managed protection",
		},
		Tips: []string{
			"Standard tier provides DDoS protection at no extra charge with Cloud Load Balancing",
			"WAF rules are charged separately from DDoS protection",
			"Consider Plus tier for enterprise-grade managed protection",
			"Use preconfigured WAF rules for common attack protection",
		},
		RelatedServices: []string{"Cloud Load Balancing", "Cloud CDN", "GKE"},
	},
	"artifact registry": {
		ServiceName:        "Artifact Registry",
		ServiceDescription: "Universal package manager for containers, language packages, and OS packages",
		Parameters: []RequiredParameter{
			{
				Name:        "region",
				Description: "Repository region or multi-region",
				Required:    true,
				Examples:    []string{"asia-northeast1", "us", "europe"},
			},
			{
				Name:        "storage_gib",
				Description: "Expected stored artifacts in GiB",
				Required:    true,
				Examples:    []string{"1", "10", "100", "500"},
				DefaultTip:  "First 0.5 GiB/month is free",
			},
			{
				Name:        "egress_gib",
				Description: "Expected egress (pulls) in GiB per month",
				Required:    true,
				Examples:    []string{"10", "100", "1000"},
				DefaultTip:  "Egress within same region to GKE/Cloud Build/Cloud Run is free",
			},
		},
		PricingFactors: []string{
			"Storage: $0.10/GiB-month (first 0.5 GiB free)",
			"Egress: Varies by destination (same region to GCP services is free)",
		},
		Tips: []string{
			"Free tier: 0.5 GiB storage/month",
			"Same-region egress to Cloud Build, GKE, Cloud Run is free",
			"Use cleanup policies to automatically delete old artifacts",
			"Consider vulnerability scanning for container images",
		},
		RelatedServices: []string{"Cloud Build", "GKE", "Cloud Run"},
	},
	"secret manager": {
		ServiceName:        "Secret Manager",
		ServiceDescription: "Secure storage and management of API keys, passwords, certificates, and other sensitive data",
		Parameters: []RequiredParameter{
			{
				Name:        "active_secret_versions",
				Description: "Number of active secret versions",
				Required:    true,
				Examples:    []string{"6 (free)", "10", "50", "100"},
				DefaultTip:  "First 6 active secret versions are free",
			},
			{
				Name:        "access_operations_per_month",
				Description: "Number of secret access operations per month",
				Required:    true,
				Examples:    []string{"10000 (free)", "100000", "1000000"},
				DefaultTip:  "First 10,000 access operations/month are free",
			},
			{
				Name:        "rotation_notifications",
				Description: "Number of rotation notifications per month",
				Required:    false,
				Examples:    []string{"3 (free)", "10", "50"},
				DefaultTip:  "First 3 rotation notifications/month are free",
			},
		},
		PricingFactors: []string{
			"Active secret versions: $0.06/version/month (first 6 free)",
			"Access operations: $0.03/10,000 operations (first 10K free)",
			"Rotation notifications: $0.05/notification (first 3/month free)",
		},
		Tips: []string{
			"Generous free tier covers many small applications",
			"Cache secrets in your application to reduce access operations",
			"Use automatic rotation for frequently changing credentials",
			"Destroy unused secret versions to reduce costs",
		},
		RelatedServices: []string{"Cloud Run", "Cloud Functions", "GKE", "Compute Engine"},
	},
}

// NewGetEstimationGuide creates a tool that provides estimation requirements for GCP services
func NewGetEstimationGuide(g *genkit.Genkit) ai.Tool {
	return genkit.DefineTool(
		g,
		"get_estimation_guide",
		`Provides a guide for what information is needed to estimate costs for ANY Google Cloud service.
IMPORTANT: Call this tool FIRST before attempting to estimate costs. This ensures you gather all necessary information from the user through conversation.

=== WORKFLOW FOR ARCHITECTURE DIAGRAMS ===
When the user provides an architecture diagram (image):
1. Analyze the diagram to identify ALL GCP services/products used
2. Call this tool for EACH identified service to get required parameters
3. Ask the user about shared parameters first (e.g., region) then service-specific details
4. Use list_services and list_skus to find correct SKU IDs for each service
5. Call estimate_cost for EACH service
6. Sum up all estimates and present a total cost breakdown

Example flow for a diagram showing "Cloud Run + Cloud SQL + Cloud Storage":
- First, confirm the region with user (likely same for all services)
- Then gather Cloud Run specs (vCPU, memory, instances, billing type)
- Then gather Cloud SQL specs (DB type, machine type, storage, HA)
- Then gather Cloud Storage specs (storage class, capacity)
- Calculate each service cost and provide a summary table with total

=== WORKFLOW FOR SINGLE SERVICE ===
For ANY service (including those not in the detailed guides):
1. Call this tool to get parameter requirements
2. Ask the user for the required information through conversation
3. Use list_services and list_skus to find specific SKUs if needed
4. Then call estimate_cost with the gathered information

=== SUPPORTED SERVICES ===
This tool works for ALL Google Cloud services:
- For common services, it provides detailed, service-specific parameter guides with pricing factors and tips.
- For any other service, it provides a comprehensive generic template with common parameters.

Services with detailed guides:
- Compute: Cloud Run, Compute Engine, GKE, Cloud Functions, App Engine
- Database: Cloud SQL, Firestore, Spanner, Memorystore
- Storage & Analytics: Cloud Storage, BigQuery
- Messaging: Pub/Sub
- Networking: Cloud CDN, Cloud Armor
- DevOps: Artifact Registry, Secret Manager`,
		func(ctx *ai.ToolContext, input GetEstimationGuideInput) (*GetEstimationGuideOutput, error) {
			log.Printf("Tool 'get_estimation_guide' called for service: %s", input.ServiceName)

			if input.ServiceName == "" {
				return nil, fmt.Errorf("service_name is required")
			}

			// Normalize service name for lookup
			normalizedName := strings.ToLower(strings.TrimSpace(input.ServiceName))

			// Try to find the service guide
			guide, found := serviceGuides[normalizedName]
			if !found {
				// Try partial matching
				for key, g := range serviceGuides {
					if strings.Contains(normalizedName, key) || strings.Contains(key, normalizedName) {
						guide = g
						found = true
						break
					}
				}
			}

			if !found {
				// Return a comprehensive generic guide for unknown services
				// This covers the common parameters needed for most GCP services
				return &GetEstimationGuideOutput{
					Guide: EstimationGuide{
						ServiceName:        input.ServiceName,
						ServiceDescription: "Detailed guide not available - using generic GCP service estimation template",
						Parameters: []RequiredParameter{
							{
								Name:        "region",
								Description: "Deployment region or location",
								Required:    true,
								Examples:    []string{"asia-northeast1 (Tokyo)", "us-central1", "europe-west1", "global"},
								DefaultTip:  "Prices vary significantly by region. Choose based on latency and compliance requirements.",
							},
							{
								Name:        "usage_type",
								Description: "How the service is billed (e.g., per hour, per request, per GB, per operation)",
								Required:    true,
								Examples:    []string{"time-based", "request-based", "storage-based", "data-processed", "per-operation"},
								DefaultTip:  "Use list_skus to discover the actual billing units for this service",
							},
							{
								Name:        "expected_usage_amount",
								Description: "Expected usage quantity per month (in appropriate unit)",
								Required:    true,
								Examples:    []string{"730 hours", "1000000 requests", "100 GB", "1000000 operations"},
							},
							{
								Name:        "tier_or_edition",
								Description: "Service tier, edition, or configuration level if applicable",
								Required:    false,
								Examples:    []string{"Standard", "Enterprise", "Basic", "Premium"},
								DefaultTip:  "Many services offer different tiers with varying features and pricing",
							},
							{
								Name:        "instance_or_resource_count",
								Description: "Number of instances, resources, or units",
								Required:    false,
								Examples:    []string{"1", "3", "10"},
							},
							{
								Name:        "high_availability",
								Description: "Whether high availability or redundancy is required",
								Required:    false,
								Examples:    []string{"true", "false"},
								DefaultTip:  "HA configurations typically cost 2-3x more",
							},
							{
								Name:        "data_transfer_egress_gb",
								Description: "Expected outbound data transfer in GB per month",
								Required:    false,
								Examples:    []string{"10", "100", "1000"},
								DefaultTip:  "Egress is often a significant cost factor",
							},
						},
						PricingFactors: []string{
							"Compute/Processing time or capacity",
							"Storage capacity and class",
							"Data transfer (especially egress)",
							"Number of operations or requests",
							"Additional features (HA, backups, encryption, etc.)",
						},
						Tips: []string{
							"IMPORTANT: Use list_services to find the service ID, then list_skus to discover available SKUs and pricing units",
							"Check Google Cloud documentation for this service's specific pricing model",
							"Many services have free tiers - verify before estimating",
							"Consider committed use discounts (CUDs) for sustained usage",
							"Regional pricing varies - check specific region costs",
						},
					},
					SuggestedQuestion: fmt.Sprintf(
						"I don't have a pre-built guide for '%s', but I can help estimate costs. "+
							"To proceed, I need to know:\n"+
							"1. **Region**: Where will this be deployed?\n"+
							"2. **Usage pattern**: How will you use it? (e.g., 24/7, on-demand, batch)\n"+
							"3. **Scale**: How much usage do you expect? (requests, storage, hours, etc.)\n"+
							"4. **Configuration**: Any specific tier, edition, or special requirements?\n\n"+
							"Alternatively, I can use list_services and list_skus to explore the available pricing options for this service. Would you like me to do that?",
						input.ServiceName,
					),
				}, nil
			}

			// Build suggested question based on required parameters
			var requiredParams []string
			for _, p := range guide.Parameters {
				if p.Required {
					requiredParams = append(requiredParams, p.Name)
				}
			}

			suggestedQuestion := fmt.Sprintf(
				"To estimate %s costs accurately, I need to know: %s. Could you provide these details?",
				guide.ServiceName,
				strings.Join(requiredParams, ", "),
			)

			return &GetEstimationGuideOutput{
				Guide:             guide,
				SuggestedQuestion: suggestedQuestion,
			}, nil
		})
}
