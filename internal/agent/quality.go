package agent

import (
	"context"
	"fmt"
	"strings"

	claudecode "github.com/yukifoo/claude-code-sdk-go"
)

type QualityReview struct {
	ComplianceStatus string
	Recommendations  string
	FullResponse     string
}

func (a *Agent) ReviewStagedCode(ctx context.Context, stagedCode, codeStandards string) (*QualityReview, error) {
	a.logger.Printf("Reviewing staged code against standards")
	a.logger.Printf("Staged code length: %d characters", len(stagedCode))
	a.logger.Printf("Code standards length: %d characters", len(codeStandards))

	prompt := fmt.Sprintf(`%s

Here is the code currently in git staging that needs to be reviewed:

<staged_code>
%s
</staged_code>

Here are the code standards that the staged code must comply with:

<code_standards>
%s
</code_standards>`, a.systemPrompt, stagedCode, codeStandards)

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(a.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatJSON),
			Verbose:        boolPtr(false),
			MaxTurns:       intPtr(10),
		},
	}

	messages, err := claudecode.QueryWithRequest(ctx, request)
	if err != nil {
		a.logger.Printf("Error reviewing staged code: %v", err)
		return nil, fmt.Errorf("review error: %w", err)
	}

	var fullResponse strings.Builder
	var complianceStatus string
	var recommendations string

	for _, message := range messages {
		for _, block := range message.Content() {
			if textBlock, ok := block.(*claudecode.TextBlock); ok {
				text := textBlock.Text
				fullResponse.WriteString(text)
				fullResponse.WriteString("\n")

				if strings.Contains(text, "<compliance_status>") {
					start := strings.Index(text, "<compliance_status>")
					end := strings.Index(text, "</compliance_status>")
					if start >= 0 && end > start {
						complianceStatus = strings.TrimSpace(text[start+19 : end])
					}
				}

				if strings.Contains(text, "<recommendations>") {
					start := strings.Index(text, "<recommendations>")
					end := strings.Index(text, "</recommendations>")
					if start >= 0 && end > start {
						recommendations = strings.TrimSpace(text[start+17 : end])
					}
				}
			}
		}
	}

	review := &QualityReview{
		ComplianceStatus: complianceStatus,
		Recommendations:  recommendations,
		FullResponse:     fullResponse.String(),
	}

	a.logger.Printf("Quality review completed. Compliance: %s", complianceStatus)

	return review, nil
}

