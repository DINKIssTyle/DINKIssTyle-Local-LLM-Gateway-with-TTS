package promptkit

import "strings"

const (
	compactTaskMarker    = "### COMPACT TASK OUTPUT ###"
	compactTaskEndMarker = "### END COMPACT TASK OUTPUT ###"
)

// BuildCompactTaskInstructions returns a small output-policy block only for
// simple transformations that commonly attract unnecessary explanations.
// The user's explicit requested format and quantity always take precedence.
func BuildCompactTaskInstructions(userText string) string {
	normalized := strings.ToLower(strings.TrimSpace(userText))
	if normalized == "" || !isCompactTransformation(normalized) {
		return ""
	}
	return "\n\n" + compactTaskMarker + "\n" +
		"This is a simple translation or rewrite. Return only the requested result, in the exact format and quantity requested. Do not add headings, explanations, alternatives, quotation marks, preambles, or follow-up offers unless the user explicitly asks for them.\n" +
		compactTaskEndMarker
}

func isCompactTransformation(normalized string) bool {
	for _, signal := range []string{
		"번역", "translate", "translation",
		"다듬", "고쳐 써", "고쳐써", "교정", "윤문",
		"rewrite", "rephrase", "proofread", "polish this", "edit this",
	} {
		if strings.Contains(normalized, signal) {
			return true
		}
	}
	return false
}
