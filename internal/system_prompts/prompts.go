package system_prompts

import (
	_ "embed"
)

var AssertCodeQuality string
var CommitExplainer string
var DebugAnalysis string
var DocumentationUpdate string
var DocumentationWrite string

func GetPrompt(name string) string {
	switch name {
	case "assert_code_quality.txt":
		return AssertCodeQuality
	case "commit_explainer.txt":
		return CommitExplainer
	case "debug_analysis.txt":
		return DebugAnalysis
	case "documentation_update.txt":
		return DocumentationUpdate
	case "documentation_write.txt":
		return DocumentationWrite
	default:
		return ""
	}
}
