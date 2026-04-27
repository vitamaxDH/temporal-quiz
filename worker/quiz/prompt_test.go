package quiz

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEasyPrompt(t *testing.T) {
	result := EasyPrompt(3, "beginner docs")
	assert.Contains(t, result, "3 easy multiple-choice")
	assert.Contains(t, result, "beginner docs")
	assert.Contains(t, result, "beginner-friendly")
	assert.Contains(t, result, `"reference":"https://docs.temporal.io/example/path"`)
	assert.NotContains(t, result, "nightmare")
}

func TestMedPrompt(t *testing.T) {
	result := MedPrompt(4, "intermediate docs")
	assert.Contains(t, result, "4 medium-difficulty")
	assert.Contains(t, result, "intermediate docs")
	assert.Contains(t, result, "PRACTICAL KNOWLEDGE")
	assert.Contains(t, result, "language-neutral Temporal concepts")
	assert.NotContains(t, result, "Rotate across these tier-1 SDKs")
}

func TestHardPrompt(t *testing.T) {
	result := HardPrompt(7, "sample docs content")
	assert.Contains(t, result, "7 hard multiple-choice")
	assert.Contains(t, result, "sample docs content")
	assert.Contains(t, result, "REAL-WORLD PRODUCTION SCENARIO")
	assert.NotContains(t, result, "nightmare")
}

func TestNightmarePrompt(t *testing.T) {
	result := NightmarePrompt(3, "advanced docs")
	assert.Contains(t, result, "3 nightmare-difficulty")
	assert.Contains(t, result, "advanced docs")
	assert.Contains(t, result, "COMPLEX PRODUCTION SCENARIOS")
	assert.True(t, strings.Contains(result, "growth, not gotchas"))
	assert.Contains(t, result, "pseudo-code")
	assert.NotContains(t, result, "Go, Java, Python, or TypeScript")
}

func TestEvalPrompt(t *testing.T) {
	result := EvalPrompt(`[{"id":"q1","question":"What is X?"}]`)
	assert.Contains(t, result, "quiz quality evaluator")
	assert.Contains(t, result, "CLARITY")
	assert.Contains(t, result, "ACCURACY")
	assert.Contains(t, result, "DIFFICULTY_FIT")
	assert.Contains(t, result, `"id":"q1"`)
	assert.Contains(t, result, "not explicitly SDK- or language-specific")
}

func TestGenerationUserMsgForCategory(t *testing.T) {
	result := GenerationUserMsgForCategory(2, "hard", "Features_Workflows", "workflow docs")

	assert.Contains(t, result, "2 hard multiple-choice")
	assert.Contains(t, result, `category "Features_Workflows"`)
	assert.Contains(t, result, "abstract SDK-specific examples")
	assert.Contains(t, result, "workflow docs")
}
