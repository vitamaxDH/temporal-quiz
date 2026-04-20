package scraper

import "strings"

// Buckets maps a bucket key to a list of token prefixes. A filename matches
// a bucket if any underscore-separated segment of the filename starts with
// any of the bucket's prefixes. This lets nested docs (e.g.
// develop_go_workflows_features.html) land in the right feature bucket
// instead of the generic Develop bucket.
//
// Priority matters: specific prefixes (e.g. "worker-versioning") are grouped
// in buckets that sit earlier in bucketOrder than the more generic
// prefixes they could otherwise collide with (e.g. "worker").
var Buckets = map[string][]string{
	// --- Core Features ---
	"Features_Workflows":               {"workflows", "workflow", "sticky-execution", "cron-job", "dynamic-handler", "parent-close-policy"},
	"Features_Activities":              {"activities", "activity", "local-activity", "heartbeat"},
	"Features_Workers":                 {"workers", "worker"},
	"Features_TaskQueues":              {"task-queue", "task-queues", "task-routing"},
	"Features_Signals_Queries_Updates": {"signals", "signal", "queries", "query", "updates", "update", "sending-messages", "handling-messages"},
	"Features_Schedules":               {"schedules", "schedule", "cron"},
	"Features_Timers_Retries":          {"retry-policies", "retry-policy", "retries", "patching", "timers", "timer"},
	"Features_Nexus":                   {"nexus"},

	// --- Execution & State ---
	"Features_History":          {"history", "event-history", "replay", "archival", "archive"},
	"Features_WorkerVersioning": {"worker-versioning", "versioning", "build-id", "deployments", "deployment-version"},
	"Features_Composition":      {"continue-as-new", "child-workflows", "child-workflow", "side-effects", "side-effect"},
	"Features_Visibility":       {"visibility", "search-attribute", "search-attributes", "list-filter", "list-filters", "dual-visibility"},
	"Features_Export":           {"export", "continuous-export", "bigquery"},

	// --- Data & Security ---
	"Features_DataConversion":    {"dataconversion", "data-conversion", "payload", "codec", "failure-converter", "remote-data"},
	"Features_Security":          {"security", "auth", "rbac", "api-key", "api-keys", "encryption", "e2e", "key-management"},
	"Features_ConnectivityRules": {"connectivity", "tls", "mtls", "private-link", "privatelink", "ip-allowlist", "ip-allowlists"},

	// --- Operations ---
	"Features_Namespaces":      {"namespaces", "namespace", "global-namespace"},
	"Operations_TemporalCloud": {"cloud"},
	"Operations_SelfHosted":    {"self-hosted", "production", "troubleshooting", "best-practices", "deployment", "ops"},

	// --- Concepts ---
	"General_Concepts": {"evaluate", "encyclopedia", "glossary", "why-temporal", "concepts", "key-concepts", "temporal-service"},

	// --- Tooling ---
	"Tooling_CLI":         {"cli", "tctl"},
	"Tooling_AI_Cookbook": {"ai-cookbook", "with-ai", "quickstarts", "quickstart"},
	"Tooling_WebUI_API":   {"web-ui", "webui", "api", "references"},

	// --- SDKs (catch-all for anything under /develop/ that didn't match a feature) ---
	"Develop": {"develop"},
}

// bucketOrder defines iteration priority. More specific prefixes come before
// more generic ones so e.g. worker-versioning.html routes to
// Features_WorkerVersioning, not Features_Workers, and dataconversion.html
// routes to Features_DataConversion, not Features_Security. Develop is last
// (catch-all inside pass 2). Tooling_WebUI_API is placed AFTER Features_Security
// so that api-key docs stay with Security instead of Tooling.
var bucketOrder = []string{
	// Most specific / distinct prefixes first
	"Features_Nexus",
	"Features_ConnectivityRules",  // tls, mtls, private-link — before Security
	"Features_WorkerVersioning",   // worker-versioning — before Workers
	"Features_Composition",        // continue-as-new, child-workflows — before Workflows
	"Features_DataConversion",     // payload, codec — before Security
	"Features_Security",           // api-key — before Tooling_WebUI_API ("api")
	"Features_Namespaces",
	"Features_Export",
	"Features_History",
	"Features_Visibility",
	"Features_Schedules",
	"Features_Timers_Retries",
	"Features_Signals_Queries_Updates",
	"Features_TaskQueues",
	"Features_Activities",
	"Features_Workflows",
	"Features_Workers",            // after WorkerVersioning
	"Operations_TemporalCloud",
	"Operations_SelfHosted",
	"General_Concepts",
	"Tooling_CLI",
	"Tooling_AI_Cookbook",
	"Tooling_WebUI_API",           // after Features_Security
	"Develop",                     // catch-all
}

// GetBucketKey routes a flattened filename (e.g. develop_go_workflows_foo.html)
// into one of the buckets above using a two-pass strategy.
//
// Pass 1: top-level match. If the whole filename OR its first segment starts
// with a prefix from any bucket EXCEPT Develop, that bucket wins. This
// anchors docs under /cloud/..., /self-hosted/..., /references/..., etc. to
// their natural home regardless of what the leaf doc is named. Example:
// cloud_namespaces.html -> Operations_TemporalCloud, not Features_Namespaces.
//
// Pass 2: segment drill-down. For anything still unmatched (typically docs
// under /develop/...), look at every underscore-separated segment and route
// to the first matching bucket. Example: develop_python_workflows.html ->
// Features_Workflows via the "workflows" segment.
//
// Develop itself is the catch-all inside pass 2 so docs like
// develop_dotnet_overview.html that have no feature segment still land
// somewhere sensible.
func GetBucketKey(filename string) string {
	name := strings.ToLower(strings.TrimSuffix(filename, ".html"))
	segments := strings.Split(name, "_")
	topSegment := segments[0]

	// Pass 1: top-level match (skip Develop so nested develop_* docs
	// aren't grabbed here before their inner feature wins below).
	for _, bucketKey := range bucketOrder {
		if bucketKey == "Develop" {
			continue
		}
		for _, prefix := range Buckets[bucketKey] {
			if strings.HasPrefix(name, prefix) || strings.HasPrefix(topSegment, prefix) {
				return bucketKey
			}
		}
	}

	// Pass 2: segment drill-down. Specific Features_* buckets are ordered
	// first in bucketOrder so they win over the generic Develop catch-all.
	for _, bucketKey := range bucketOrder {
		for _, prefix := range Buckets[bucketKey] {
			for _, seg := range segments {
				if strings.HasPrefix(seg, prefix) {
					return bucketKey
				}
			}
		}
	}

	return "Miscellaneous"
}

// SortedBucketKeys returns the full list of bucket keys in display order
// (alphabetical, with Miscellaneous at the end). Consumed by the scraper
// when writing bucket text files.
func SortedBucketKeys() []string {
	return []string{
		"Develop",
		"Features_Activities",
		"Features_ConnectivityRules",
		"Features_Composition",
		"Features_DataConversion",
		"Features_Export",
		"Features_History",
		"Features_Namespaces",
		"Features_Nexus",
		"Features_Schedules",
		"Features_Security",
		"Features_Signals_Queries_Updates",
		"Features_TaskQueues",
		"Features_Timers_Retries",
		"Features_Visibility",
		"Features_WorkerVersioning",
		"Features_Workers",
		"Features_Workflows",
		"General_Concepts",
		"Operations_SelfHosted",
		"Operations_TemporalCloud",
		"Tooling_AI_Cookbook",
		"Tooling_CLI",
		"Tooling_WebUI_API",
		"Miscellaneous",
	}
}
