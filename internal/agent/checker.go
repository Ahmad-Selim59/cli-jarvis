package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	claudecode "github.com/yukifoo/claude-code-sdk-go"
)

type TopicMatch struct {
	Topic        string
	ExistingFile string 
	IsMatch      bool
}

func (a *Agent) CheckExistingDocs(ctx context.Context, topics []string) ([]TopicMatch, error) {
	docsDir := filepath.Join(a.folder, "documentation")
	
	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan documentation directory: %w", err)
	}

	if len(files) == 0 {
		matches := make([]TopicMatch, len(topics))
		for i, topic := range topics {
			matches[i] = TopicMatch{
				Topic:        topic,
				ExistingFile: "",
				IsMatch:      false,
			}
		}
		return matches, nil
	}

	var fileList strings.Builder
	for _, file := range files {
		fileList.WriteString(fmt.Sprintf("- %s\n", filepath.Base(file)))
	}

	var topicsList strings.Builder
	for i, topic := range topics {
		topicsList.WriteString(fmt.Sprintf("%d. %s\n", i+1, topic))
	}

	prompt := fmt.Sprintf(`You are analyzing a documentation directory to match requested topics with existing documentation files.

Existing documentation files in %s/documentation/:
%s

Topics the user wants to document:
%s

For each topic, determine if there's already an existing documentation file that covers it. A match means the file documents the same subject/feature, even if the filename is slightly different.

Respond with ONLY a JSON array in this exact format, no other text:
[
  {"topic": "topic name", "existing_file": "filename.md", "is_match": true},
  {"topic": "topic name", "existing_file": "", "is_match": false}
]

Rules:
- Use the exact topic names from the list above
- For existing_file, use only the filename (not full path)
- Set is_match to true only if you're confident the file covers that topic
- If no match exists, set existing_file to empty string and is_match to false
- Return ONLY the JSON array, no explanations`, a.folder, fileList.String(), topicsList.String())

	a.logger.Printf("Checking existing documentation for %d topics", len(topics))

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read", "LS"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(a.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatJSON),
			Verbose:        boolPtr(false),
			MaxTurns:       intPtr(3), // Quick check, don't need many turns
		},
	}

	messages, err := claudecode.QueryWithRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing docs: %w", err)
	}

	var jsonResponse string
	for _, message := range messages {
		for _, block := range message.Content() {
			if textBlock, ok := block.(*claudecode.TextBlock); ok {
				text := strings.TrimSpace(textBlock.Text)
				
				if strings.Contains(text, "```json") {
					start := strings.Index(text, "```json")
					end := strings.Index(text[start+7:], "```")
					if start >= 0 && end > 0 {
						jsonResponse = strings.TrimSpace(text[start+7 : start+7+end])
						break
					}
				}
				
				if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
					jsonResponse = text
					break
				}
			}
		}
		if jsonResponse != "" {
			break
		}
	}

	if jsonResponse == "" {
		a.logger.Printf("ERROR: Could not extract JSON from Claude response")
		return nil, fmt.Errorf("Claude did not return expected JSON response")
	}

	a.logger.Printf("Found JSON response, length: %d", len(jsonResponse))

	type jsonMatch struct {
		Topic        string `json:"topic"`
		ExistingFile string `json:"existing_file"`
		IsMatch      bool   `json:"is_match"`
	}

	var jsonMatches []jsonMatch
	err = json.Unmarshal([]byte(jsonResponse), &jsonMatches)
	if err != nil {
		a.logger.Printf("JSON parse error: %v", err)
		a.logger.Printf("JSON content: %s", jsonResponse)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	matches := make([]TopicMatch, len(jsonMatches))
	for i, jm := range jsonMatches {
		matches[i] = TopicMatch{
			Topic:        jm.Topic,
			ExistingFile: jm.ExistingFile,
			IsMatch:      jm.IsMatch,
		}
	}

	a.logger.Printf("Successfully parsed %d topic matches", len(matches))
	return matches, nil
}

