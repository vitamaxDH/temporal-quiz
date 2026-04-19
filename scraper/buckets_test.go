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
		{"develop_go_workers.html", "Develop"},
		{"develop_java_activities.html", "Develop"},
		{"develop_python_workflows.html", "Develop"},
		{"develop_typescript_signals.html", "Develop"},
		{"develop_dotnet_overview.html", "Develop"},
		{"develop_php_activities.html", "Develop"},
		{"develop_ruby_workers.html", "Develop"},
		{"develop_index.html", "Develop"},
		{"workflows_overview.html", "Features_Workflows"},
		{"workflow-timeouts.html", "Features_Workflows"},
		{"child-workflows.html", "Features_Workflows"},
		{"activities_overview.html", "Features_Activities"},
		{"activity-heartbeats.html", "Features_Activities"},
		{"workers_overview.html", "Features_Workers_and_Routing"},
		{"task-queue-routing.html", "Features_Workers_and_Routing"},
		{"namespaces.html", "Features_Workers_and_Routing"},
		{"dataconversion.html", "Features_Data_and_Security"},
		{"security.html", "Features_Data_and_Security"},
		{"nexus.html", "Features_Nexus"},
		{"sending-messages.html", "Features_Messaging_and_Visibility"},
		{"visibility.html", "Features_Messaging_and_Visibility"},
		{"schedule.html", "Features_Other"},
		{"retry-policies.html", "Features_Other"},
		{"self-hosted-guide.html", "Self_Hosted_and_Ops"},
		{"cloud-namespaces.html", "Temporal_Cloud"},
		{"ai-cookbook-intro.html", "AI_and_Cookbook"},
		{"cli-reference.html", "CLI_and_References"},
		{"tags-overview.html", "Tags"},
		{"evaluate.html", "Evaluate_and_Concepts"},
		{"encyclopedia.html", "Evaluate_and_Concepts"},
		{"glossary.html", "Evaluate_and_Concepts"},
		{"random-page.html", "Miscellaneous"},
		{"index.html", "Evaluate_and_Concepts"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := GetBucketKey(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}
