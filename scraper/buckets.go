package scraper

import "strings"

// Buckets maps a bucket key to a list of token prefixes. A filename matches
// a bucket if any underscore-separated segment of the filename starts with
// any of the bucket's prefixes. This lets nested docs (e.g.
// develop_go_workflows_features.html) land in the right feature bucket
// instead of the generic Develop bucket.
var Buckets = map[string][]string{
	"Features_Workflows":                {"workflows", "workflow", "child-workflow", "sticky-execution", "cron-job", "parent-close-policy", "dynamic-handler", "continue-as-new", "side-effect", "versioning", "replay"},
	"Features_Activities":               {"activities", "activity", "local-activity", "heartbeat"},
	"Features_Workers_and_Routing":      {"workers", "worker", "task-queue", "task-routing", "namespaces", "global-namespace", "sticky-execution"},
	"Features_Data_and_Security":        {"dataconversion", "data-conversion", "payload", "codec", "failure-converter", "remote-data", "security", "key-management", "auth", "tls", "mtls"},
	"Features_Nexus":                    {"nexus"},
	"Features_Messaging_and_Visibility": {"sending-messages", "handling-messages", "queries", "signals", "update", "updates", "visibility", "search-attribute", "search-attributes", "list-filter", "dual-visibility"},
	"Features_Other":                    {"schedule", "schedules", "retry-policies", "retry-policy", "patching"},
	"Evaluate_and_Concepts":             {"evaluate", "encyclopedia", "glossary", "temporal-service", "why-temporal", "concepts", "key-concepts"},
	"Temporal_Cloud":                    {"cloud"},
	"Self_Hosted_and_Ops":               {"self-hosted", "production", "ops", "troubleshooting", "best-practices", "deployment"},
	"AI_and_Cookbook":                   {"ai-cookbook", "with-ai", "quickstarts", "quickstart"},
	"CLI_and_References":                {"cli", "tctl", "references", "api", "web-ui"},
	"Develop":                           {"develop"},
	"Tags":                              {"tags"},
}

// bucketOrder defines iteration order. Specific Features_* buckets come
// BEFORE the generic Develop catch-all so a nested doc like
// develop_go_workflows_basics.html is routed to Features_Workflows, not
// Develop. Miscellaneous is the implicit fallback.
var bucketOrder = []string{
	"Features_Workflows",
	"Features_Activities",
	"Features_Workers_and_Routing",
	"Features_Data_and_Security",
	"Features_Nexus",
	"Features_Messaging_and_Visibility",
	"Features_Other",
	"Evaluate_and_Concepts",
	"Temporal_Cloud",
	"Self_Hosted_and_Ops",
	"AI_and_Cookbook",
	"CLI_and_References",
	"Develop",
	"Tags",
}

// GetBucketKey routes a flattened filename (e.g. develop_go_workflows_foo.html)
// into one of the buckets above using a two-pass strategy.
//
// Pass 1: top-level match. If the whole filename OR its first segment starts
// with a prefix from any bucket EXCEPT Develop, that bucket wins. This
// anchors docs under /cloud/..., /self-hosted/..., /references/..., etc. to
// their natural home regardless of what the leaf doc is named. Example:
// cloud_namespaces.html -> Temporal_Cloud, not Features_Workers_and_Routing.
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

	// Pass 2: segment drill-down. Features_* buckets sit first in
	// bucketOrder so they win over the generic Develop catch-all.
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

func SortedBucketKeys() []string {
	return []string{
		"AI_and_Cookbook",
		"CLI_and_References",
		"Develop",
		"Evaluate_and_Concepts",
		"Features_Activities",
		"Features_Data_and_Security",
		"Features_Messaging_and_Visibility",
		"Features_Nexus",
		"Features_Other",
		"Features_Workers_and_Routing",
		"Features_Workflows",
		"Miscellaneous",
		"Self_Hosted_and_Ops",
		"Tags",
		"Temporal_Cloud",
	}
}
