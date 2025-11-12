package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	claudecode "github.com/yukifoo/claude-code-sdk-go"
)

type Agent struct {
	systemPrompt string
	folder       string
	logger       *log.Logger
}

type ProcessResult struct {
	FileName string
	Success  bool
	Error    error
}

func New(systemPromptContent, folder string) (*Agent, error) {
	systemPrompt := systemPromptContent
	systemPrompt += fmt.Sprintf("\n\nHere is the codebase path where you should look for the relevant code files:\n<codebase_path>\n%s\n</codebase_path>", folder)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".docu-jarvis", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, "docu-jarvis.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	logger := log.New(logFile, "", log.LstdFlags)

	return &Agent{
		systemPrompt: systemPrompt,
		folder:       folder,
		logger:       logger,
	}, nil
}

func (a *Agent) ProcessFile(ctx context.Context, filePath string) error {
	fileName := filepath.Base(filePath)

	prompt := fmt.Sprintf(`%s

Here is the documentation file that you need to analyze:

<documentation>
%s/documentation/%s
</documentation>
`, a.systemPrompt, a.folder, fileName)

	a.logger.Printf("Starting processing: %s", fileName)
	a.logger.Printf("Prompt length: %d characters", len(prompt))

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read", "Write"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(a.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatStreamJSON),
			Verbose:        boolPtr(false),
		},
	}

	messageChan, errorChan := claudecode.QueryStreamWithRequest(ctx, request)

	messageCount := 0
	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				a.logger.Printf("Completed processing: %s (received %d messages)", fileName, messageCount)
				return nil
			}

			messageCount++
			a.logMessage(fileName, message)

		case err := <-errorChan:
			if err != nil {
				a.logger.Printf("Error processing %s: %v", fileName, err)
				return fmt.Errorf("streaming error: %w", err)
			}

		case <-ctx.Done():
			a.logger.Printf("Context cancelled for %s", fileName)
			return ctx.Err()
		}
	}
}

func (a *Agent) ProcessDocuments(ctx context.Context) (int, int, error) {
	docsDir := filepath.Join(a.folder, "documentation")

	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		return 0, 0, fmt.Errorf("documentation directory does not exist: %s", docsDir)
	}

	files, err := filepath.Glob(filepath.Join(docsDir, "*.md"))
	if err != nil {
		return 0, 0, fmt.Errorf("failed to glob markdown files: %w", err)
	}

	if len(files) == 0 {
		return 0, 0, fmt.Errorf("no .md files found in: %s", docsDir)
	}

	totalFiles := len(files)
	a.logger.Printf("Found %d markdown files to process", totalFiles)
	fmt.Printf("Processing %d documentation files concurrently...\n", totalFiles)

	resultChan := make(chan ProcessResult, totalFiles)
	var wg sync.WaitGroup

	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			fileName := filepath.Base(path)
			fmt.Printf("  → Started: %s\n", fileName)

			err := a.ProcessFile(ctx, path)

			result := ProcessResult{
				FileName: fileName,
				Success:  err == nil,
				Error:    err,
			}

			resultChan <- result

			if err == nil {
				fmt.Printf("  ✓ Completed: %s\n", fileName)
			} else {
				fmt.Printf("  ✗ Failed: %s - %v\n", fileName, err)
			}
		}(filePath)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	successCount := 0
	var failedFiles []string

	for result := range resultChan {
		if result.Success {
			successCount++
		} else {
			failedFiles = append(failedFiles, result.FileName)
		}
	}

	a.logger.Printf("Processing complete: %d/%d succeeded", successCount, totalFiles)
	if len(failedFiles) > 0 {
		a.logger.Printf("Failed files: %v", failedFiles)
	}

	fmt.Printf("\nSummary: %d/%d files processed successfully\n", successCount, totalFiles)

	return successCount, totalFiles, nil
}

func (a *Agent) UpdateSpecificDocuments(ctx context.Context, filePaths []string) (int, int, error) {
	if len(filePaths) == 0 {
		return 0, 0, nil
	}

	for _, path := range filePaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return 0, 0, fmt.Errorf("file does not exist: %s", path)
		}
	}

	totalFiles := len(filePaths)
	a.logger.Printf("Updating %d specific markdown files", totalFiles)
	fmt.Printf("Updating %d documentation files concurrently...\n", totalFiles)

	resultChan := make(chan ProcessResult, totalFiles)
	var wg sync.WaitGroup

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			fileName := filepath.Base(path)
			fmt.Printf("  → Started: %s\n", fileName)

			err := a.ProcessFile(ctx, path)

			result := ProcessResult{
				FileName: fileName,
				Success:  err == nil,
				Error:    err,
			}

			resultChan <- result

			if err == nil {
				fmt.Printf("  ✓ Completed: %s\n", fileName)
			} else {
				fmt.Printf("  ✗ Failed: %s - %v\n", fileName, err)
			}
		}(filePath)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	successCount := 0
	var failedFiles []string

	for result := range resultChan {
		if result.Success {
			successCount++
		} else {
			failedFiles = append(failedFiles, result.FileName)
		}
	}

	a.logger.Printf("Update complete: %d/%d succeeded", successCount, totalFiles)
	if len(failedFiles) > 0 {
		a.logger.Printf("Failed files: %v", failedFiles)
	}

	fmt.Printf("\nSummary: %d/%d files updated successfully\n", successCount, totalFiles)

	return successCount, totalFiles, nil
}

func (a *Agent) logMessage(fileName string, msg claudecode.Message) {
	msgType := msg.Type()

	switch msgType {
	case claudecode.MessageTypeUser, claudecode.MessageTypeAssistant:
		for _, block := range msg.Content() {
			switch b := block.(type) {
			case *claudecode.TextBlock:
				// Log first 100 chars of text to avoid huge logs
				text := b.Text
				if len(text) > 100 {
					text = text[:100] + "..."
				}
				a.logger.Printf("[%s] %s: %s", fileName, msgType, text)

			case *claudecode.ToolUseBlock:
				a.logger.Printf("[%s] Tool use: %s (ID: %s)", fileName, b.Name, b.ID)

			case *claudecode.ToolResultBlock:
				a.logger.Printf("[%s] Tool result (ID: %s)", fileName, b.ToolUseID)
			}
		}

	case claudecode.MessageTypeSystem:
		if sysMsg, ok := msg.(*claudecode.SystemMessage); ok {
			a.logger.Printf("[%s] System - Session: %s", fileName, sysMsg.SessionID)
		}

	case claudecode.MessageTypeResult:
		if resultMsg, ok := msg.(*claudecode.ResultMessage); ok {
			a.logger.Printf("[%s] Result - Duration: %dms, Turns: %d, Success: %v",
				fileName, resultMsg.DurationMs, resultMsg.NumTurns, !resultMsg.IsError)

			if resultMsg.Usage != nil {
				a.logger.Printf("[%s] Tokens - Input: %d, Output: %d",
					fileName, resultMsg.Usage.InputTokens, resultMsg.Usage.OutputTokens)
			}
		}
	}
}

func (a *Agent) WriteTopic(ctx context.Context, topic string) error {
	a.logger.Printf("Starting documentation writing for topic: %s", topic)

	prompt := fmt.Sprintf(`%s

The topic you need to document is: %s

The codebase you will be reading through is located at: %s

IMPORTANT: You must write the documentation file in the documentation/ folder within the codebase directory.
Create a markdown file with an appropriate filename based on the topic (e.g., "api-authentication.md", "database-schema.md").
The documentation should be saved to: %s/documentation/

Please analyze the codebase and create comprehensive documentation for this topic following the structure and guidelines provided in the system prompt.`, a.systemPrompt, topic, a.folder, a.folder)

	a.logger.Printf("Topic: %s - Prompt length: %d characters", topic, len(prompt))

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read", "Write", "LS", "Grep"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(a.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatJSON),
			Verbose:        boolPtr(false),
		},
	}

	// Use non-streaming query to avoid buffer overflow
	messages, err := claudecode.QueryWithRequest(ctx, request)
	if err != nil {
		a.logger.Printf("Error writing documentation for topic %s: %v", topic, err)
		return fmt.Errorf("query error: %w", err)
	}

	a.logger.Printf("Completed writing documentation for topic: %s (received %d messages)", topic, len(messages))
	for _, message := range messages {
		a.logTopicMessage(topic, message)
	}

	return nil
}

func (a *Agent) WriteDocumentation(ctx context.Context, topics []string) (int, int, error) {
	totalTopics := len(topics)
	a.logger.Printf("Starting documentation writing for %d topics", totalTopics)

	docsDir := filepath.Join(a.folder, "documentation")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create documentation directory: %w", err)
	}
	a.logger.Printf("Documentation directory ready: %s", docsDir)

	fmt.Printf("Writing documentation for %d topics concurrently...\n", totalTopics)

	resultChan := make(chan ProcessResult, totalTopics)
	var wg sync.WaitGroup

	for _, topic := range topics {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()

			fmt.Printf("  → Started: %s\n", t)

			err := a.WriteTopic(ctx, t)

			result := ProcessResult{
				FileName: t,
				Success:  err == nil,
				Error:    err,
			}

			resultChan <- result

			if err == nil {
				fmt.Printf("  ✓ Completed: %s\n", t)
			} else {
				fmt.Printf("  ✗ Failed: %s - %v\n", t, err)
			}
		}(topic)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	successCount := 0
	var failedTopics []string

	for result := range resultChan {
		if result.Success {
			successCount++
		} else {
			failedTopics = append(failedTopics, result.FileName)
		}
	}

	a.logger.Printf("Documentation writing complete: %d/%d succeeded", successCount, totalTopics)
	if len(failedTopics) > 0 {
		a.logger.Printf("Failed topics: %v", failedTopics)
	}

	fmt.Printf("\nSummary: %d/%d topics documented successfully\n", successCount, totalTopics)

	return successCount, totalTopics, nil
}

func (a *Agent) logTopicMessage(topic string, msg claudecode.Message) {
	msgType := msg.Type()

	switch msgType {
	case claudecode.MessageTypeUser, claudecode.MessageTypeAssistant:
		for _, block := range msg.Content() {
			switch b := block.(type) {
			case *claudecode.TextBlock:
				text := b.Text
				if len(text) > 100 {
					text = text[:100] + "..."
				}
				a.logger.Printf("[%s] %s: %s", topic, msgType, text)

			case *claudecode.ToolUseBlock:
				a.logger.Printf("[%s] Tool use: %s (ID: %s)", topic, b.Name, b.ID)

			case *claudecode.ToolResultBlock:
				a.logger.Printf("[%s] Tool result (ID: %s)", topic, b.ToolUseID)
			}
		}

	case claudecode.MessageTypeSystem:
		if sysMsg, ok := msg.(*claudecode.SystemMessage); ok {
			a.logger.Printf("[%s] System - Session: %s", topic, sysMsg.SessionID)
		}

	case claudecode.MessageTypeResult:
		if resultMsg, ok := msg.(*claudecode.ResultMessage); ok {
			a.logger.Printf("[%s] Result - Duration: %dms, Turns: %d, Success: %v",
				topic, resultMsg.DurationMs, resultMsg.NumTurns, !resultMsg.IsError)

			if resultMsg.Usage != nil {
				a.logger.Printf("[%s] Tokens - Input: %d, Output: %d",
					topic, resultMsg.Usage.InputTokens, resultMsg.Usage.OutputTokens)
			}
		}
	}
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func outputFormatPtr(f claudecode.OutputFormat) *claudecode.OutputFormat {
	return &f
}
