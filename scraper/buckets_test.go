package scraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBucketKey(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		// Nested /develop/<sdk>/<feature> docs drill down to the Features_*
		// bucket via segment matching. The SDK segment is ignored.
		{"develop_go_workers.html", "Features_Workers"},
		{"develop_java_activities.html", "Features_Activities"},
		{"develop_python_workflows.html", "Features_Workflows"},
		{"develop_typescript_signals.html", "Features_Signals_Queries_Updates"},
		{"develop_go_schedules.html", "Features_Schedules"},
		{"develop_java_worker-versioning.html", "Features_WorkerVersioning"},
		{"develop_go_workflows_continue-as-new.html", "Features_Composition"},
		{"develop_python_history_replay.html", "Features_History"},
		{"develop_php_activities.html", "Features_Activities"},
		{"develop_ruby_workers.html", "Features_Workers"},
		// Pure Develop docs with no feature segment fall through to Develop.
		{"develop_dotnet_overview.html", "Develop"},
		{"develop_index.html", "Develop"},

		// Core features (flat filenames)
		{"workflows_overview.html", "Features_Workflows"},
		{"workflow-timeouts.html", "Features_Workflows"},
		{"child-workflows.html", "Features_Composition"},
		{"continue-as-new.html", "Features_Composition"},
		{"activities_overview.html", "Features_Activities"},
		{"activity-heartbeats.html", "Features_Activities"},
		{"workers_overview.html", "Features_Workers"},
		{"worker-versioning.html", "Features_WorkerVersioning"},
		{"task-queue-routing.html", "Features_TaskQueues"},
		{"nexus.html", "Features_Nexus"},
		{"schedule.html", "Features_Schedules"},
		{"schedules_overview.html", "Features_Schedules"},
		{"retry-policies.html", "Features_Timers_Retries"},
		{"timers.html", "Features_Timers_Retries"},
		{"sending-messages.html", "Features_Signals_Queries_Updates"},
		{"signals.html", "Features_Signals_Queries_Updates"},
		{"updates.html", "Features_Signals_Queries_Updates"},

		// Execution & state
		{"history.html", "Features_History"},
		{"event-history.html", "Features_History"},
		{"replay.html", "Features_History"},
		{"visibility.html", "Features_Visibility"},
		{"search-attribute.html", "Features_Visibility"},
		{"export-bigquery.html", "Features_Export"},
		{"continuous-export.html", "Features_Export"},

		// Data & security
		{"dataconversion.html", "Features_DataConversion"},
		{"payload-codec.html", "Features_DataConversion"},
		{"security.html", "Features_Security"},
		{"api-key-auth.html", "Features_Security"},
		{"tls-configuration.html", "Features_ConnectivityRules"},
		{"mtls.html", "Features_ConnectivityRules"},
		{"private-link.html", "Features_ConnectivityRules"},

		// Operations
		{"namespaces.html", "Features_Namespaces"},
		{"self-hosted-guide.html", "Operations_SelfHosted"},
		{"cloud-namespaces.html", "Operations_TemporalCloud"},

		// Tooling / concepts
		{"ai-cookbook-intro.html", "Tooling_AI_Cookbook"},
		{"cli-reference.html", "Tooling_CLI"},
		{"web-ui-overview.html", "Tooling_WebUI_API"},
		{"evaluate.html", "General_Concepts"},
		{"encyclopedia.html", "General_Concepts"},
		{"glossary.html", "General_Concepts"},

		// Catch-all
		{"random-page.html", "Miscellaneous"},
		{"tags-overview.html", "Miscellaneous"}, // Tags bucket retired
		{"index.html", "Miscellaneous"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := GetBucketKey(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}
