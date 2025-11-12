package agent

import (
	"context"
	"fmt"
	"strings"

	claudecode "github.com/yukifoo/claude-code-sdk-go"
)

type CommitAnalysis struct {
	CommitHash  string
	CommitMsg   string
	Author      string
	Date        string
	Explanation string
	IsLikely    bool
	Confidence  int // 0-100
}

type CommitAnalysisResult struct {
	Commit   string
	Analysis *CommitAnalysis
	Error    error
}

func (a *Agent) AnalyzeSingleCommit(ctx context.Context, commit, bugDescription string) (*CommitAnalysis, error) {
	// Parse commit info: hash|author|date|subject
	parts := strings.Split(commit, "|")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid commit format")
	}

	commitHash := parts[0]
	commitAuthor := parts[1]
	commitDate := parts[2]
	commitMsg := parts[3]

	a.logger.Printf("Analyzing commit %s for bug", commitHash[:8])

	prompt := fmt.Sprintf(`%s

Codebase location: %s

Commit to analyze:
- Hash: %s
- Author: %s
- Date: %s
- Message: %s

Bug description:
%s`, a.systemPrompt, a.folder, commitHash, commitAuthor, commitDate, commitMsg, bugDescription)

	a.logger.Printf("Debug analysis prompt length: %d characters", len(prompt))

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read", "Grep", "LS"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(a.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatJSON),
			Verbose:        boolPtr(false),
			MaxTurns:       intPtr(25), 
		},
	}

	messages, err := claudecode.QueryWithRequest(ctx, request)
	if err != nil {
		a.logger.Printf("Error analyzing commits: %v", err)
		return nil, fmt.Errorf("analysis error: %w", err)
	}

	var jsonResponse string
	for _, message := range messages {
		for _, block := range message.Content() {
			if textBlock, ok := block.(*claudecode.TextBlock); ok {
				text := strings.TrimSpace(textBlock.Text)

				// Handle markdown code blocks
				if strings.Contains(text, "```json") {
					start := strings.Index(text, "```json")
					end := strings.Index(text[start+7:], "```")
					if start >= 0 && end > 0 {
						jsonResponse = strings.TrimSpace(text[start+7 : start+7+end])
						break
					}
				}

				// Handle plain JSON objects
				if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
					jsonResponse = text
					break
				}

				// Try to extract JSON from anywhere in text
				startIdx := strings.Index(text, "{")
				endIdx := strings.LastIndex(text, "}")
				if startIdx >= 0 && endIdx > startIdx {
					potentialJSON := strings.TrimSpace(text[startIdx : endIdx+1])
					if strings.HasPrefix(potentialJSON, "{") && strings.HasSuffix(potentialJSON, "}") {
						jsonResponse = potentialJSON
						break
					}
				}
			}
		}
		if jsonResponse != "" {
			break
		}
	}

	if jsonResponse == "" {
		a.logger.Printf("ERROR: Could not extract JSON from debug analysis")
		return nil, fmt.Errorf("Claude did not return expected JSON response")
	}

	a.logger.Printf("Found JSON response, length: %d", len(jsonResponse))

	analysis := &CommitAnalysis{}

	jsonResponse = strings.TrimSpace(jsonResponse)
	jsonResponse = strings.Trim(jsonResponse, "{}")

	pairs := splitJSONPairs(jsonResponse)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.Trim(strings.TrimSpace(parts[0]), "\"")
		value := strings.TrimSpace(parts[1])

		value = strings.TrimSuffix(value, ",")
		value = strings.Trim(value, "\"")

		switch key {
		case "commit_hash":
			analysis.CommitHash = value
		case "commit_message":
			analysis.CommitMsg = value
		case "author":
			analysis.Author = value
		case "date":
			analysis.Date = value
		case "explanation":
			analysis.Explanation = value
		case "is_likely":
			analysis.IsLikely = value == "true"
		case "confidence":
			fmt.Sscanf(value, "%d", &analysis.Confidence)
		}
	}

	a.logger.Printf("Parsed commit analysis: hash=%s, likely=%v, confidence=%d", analysis.CommitHash, analysis.IsLikely, analysis.Confidence)

	return analysis, nil
}

func (a *Agent) AnalyzeBugInCommits(ctx context.Context, commits []string, bugDescription string) (*CommitAnalysis, error) {
	a.logger.Printf("Analyzing %d commits concurrently for bug: %s", len(commits), bugDescription)
	
	totalCommits := len(commits)
	resultChan := make(chan CommitAnalysisResult, totalCommits)
	
	for _, commit := range commits {
		go func(c string) {
			analysis, err := a.AnalyzeSingleCommit(ctx, c, bugDescription)
			
			result := CommitAnalysisResult{
				Commit:   c,
				Analysis: analysis,
				Error:    err,
			}
			
			resultChan <- result
		}(commit)
	}
	
	var analyses []*CommitAnalysis
	completed := 0
	
	for completed < totalCommits {
		select {
		case result := <-resultChan:
			completed++
			fmt.Printf("\r  Analyzed: %d/%d commits", completed, totalCommits)
			
			if result.Error != nil {
				a.logger.Printf("Error analyzing commit: %v", result.Error)
				continue
			}
			
			if result.Analysis != nil {
				analyses = append(analyses, result.Analysis)
			}
			
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	fmt.Println()
	
	if len(analyses) == 0 {
		return nil, fmt.Errorf("no commits could be analyzed")
	}
	
	var bestMatch *CommitAnalysis
	for _, analysis := range analyses {
		if analysis.IsLikely {
			if bestMatch == nil || analysis.Confidence > bestMatch.Confidence {
				bestMatch = analysis
			}
		}
	}
	
	if bestMatch == nil {
		for _, analysis := range analyses {
			if bestMatch == nil || analysis.Confidence > bestMatch.Confidence {
				bestMatch = analysis
			}
		}
	}
	
	a.logger.Printf("Best match found: commit=%s, confidence=%d", bestMatch.CommitHash, bestMatch.Confidence)
	return bestMatch, nil
}

func splitJSONPairs(jsonContent string) []string {
	var pairs []string
	var current strings.Builder
	inQuotes := false
	depth := 0

	for i := 0; i < len(jsonContent); i++ {
		char := jsonContent[i]

		switch char {
		case '"':
			if i == 0 || jsonContent[i-1] != '\\' {
				inQuotes = !inQuotes
			}
			current.WriteByte(char)
		case '{', '[':
			if !inQuotes {
				depth++
			}
			current.WriteByte(char)
		case '}', ']':
			if !inQuotes {
				depth--
			}
			current.WriteByte(char)
		case ',':
			if !inQuotes && depth == 0 {
				pairs = append(pairs, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(char)
			}
		default:
			current.WriteByte(char)
		}
	}

	if current.Len() > 0 {
		pairs = append(pairs, strings.TrimSpace(current.String()))
	}

	return pairs
}

