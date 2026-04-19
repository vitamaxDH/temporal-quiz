package scraper

import "strings"

var Buckets = map[string][]string{
	"Evaluate_and_Concepts":              {"evaluate", "encyclopedia", "glossary", "index", "temporal.html", "temporal-service"},
	"Develop":                            {"develop"},
	"Features_Workflows":                 {"workflows", "workflow-", "child-workflows", "sticky-execution", "cron-job", "parent-close-policy", "dynamic-handler"},
	"Features_Activities":                {"activities", "activity-", "local-activity"},
	"Features_Workers_and_Routing":       {"workers", "worker-", "task-queue", "task-routing", "namespaces", "global-namespace"},
	"Features_Data_and_Security":         {"dataconversion", "payload", "codec", "failure-converter", "remote-data", "security", "key-management", "auth"},
	"Features_Nexus":                     {"nexus"},
	"Features_Messaging_and_Visibility":  {"sending-messages", "handling-messages", "queries", "signals", "visibility", "search-attribute", "list-filter", "dual-visibility"},
	"Features_Other":                     {"schedule", "retry-policies", "patching"},
	"Self_Hosted_and_Ops":                {"self-hosted", "production", "ops", "troubleshooting", "best-practices"},
	"Temporal_Cloud":                     {"cloud"},
	"AI_and_Cookbook":                     {"ai-cookbook", "with-ai", "quickstarts"},
	"CLI_and_References":                 {"cli", "tctl", "references", "api", "web-ui"},
	"Tags":                               {"tags"},
}

// bucketOrder defines iteration order (Go maps are unordered).
var bucketOrder = []string{
	"Evaluate_and_Concepts",
	"Develop",
	"Features_Workflows",
	"Features_Activities",
	"Features_Workers_and_Routing",
	"Features_Data_and_Security",
	"Features_Nexus",
	"Features_Messaging_and_Visibility",
	"Features_Other",
	"Self_Hosted_and_Ops",
	"Temporal_Cloud",
	"AI_and_Cookbook",
	"CLI_and_References",
	"Tags",
}

func GetBucketKey(filename string) string {
	name := strings.ToLower(strings.TrimSuffix(filename, ".html"))

	for _, bucketKey := range bucketOrder {
		for _, prefix := range Buckets[bucketKey] {
			if strings.HasPrefix(name, prefix) {
				return bucketKey
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
