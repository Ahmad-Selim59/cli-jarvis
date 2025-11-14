package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	claudecode "github.com/yukifoo/claude-code-sdk-go"
)

type ConversationMessage struct {
	Role    string
	Content string
}

type CommitExplainer struct {
	agent               *Agent
	commitHash          string
	commitDiff          string
	conversationHistory []ConversationMessage
}

func NewCommitExplainer(agent *Agent, commitHash, commitDiff string) *CommitExplainer {
	return &CommitExplainer{
		agent:               agent,
		commitHash:          commitHash,
		commitDiff:          commitDiff,
		conversationHistory: []ConversationMessage{},
	}
}

func (ce *CommitExplainer) StartConversation(ctx context.Context, initialQuestion string) error {
	ce.agent.logger.Printf("Starting commit explanation conversation for commit: %s", ce.commitHash)

	if initialQuestion != "" {
		fmt.Printf("\n> %s\n\n", initialQuestion)
		ce.conversationHistory = append(ce.conversationHistory, ConversationMessage{
			Role:    "user",
			Content: initialQuestion,
		})

		fmt.Print("Claude: ")
		_, err := ce.getResponse(ctx)
		if err != nil {
			return err
		}

		fmt.Println()
	} else {
		initialPrompt := "Please provide a comprehensive explanation of this commit. What changes were made and why?"
		ce.conversationHistory = append(ce.conversationHistory, ConversationMessage{
			Role:    "user",
			Content: initialPrompt,
		})

		fmt.Print("Claude: ")
		_, err := ce.getResponse(ctx)
		if err != nil {
			return err
		}

		fmt.Println()
	}

	return ce.interactiveLoop(ctx)
}

func (ce *CommitExplainer) interactiveLoop(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("Interactive conversation mode - Ask questions about the commit")
	fmt.Println("Type 'exit', 'quit', or press Ctrl+C to end the conversation")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	for {
		fmt.Print("You: ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		userInput = strings.TrimSpace(userInput)

		if userInput == "" {
			continue
		}
		if strings.ToLower(userInput) == "exit" || strings.ToLower(userInput) == "quit" {
			fmt.Println("\n✓ Conversation ended")
			return nil
		}

		ce.conversationHistory = append(ce.conversationHistory, ConversationMessage{
			Role:    "user",
			Content: userInput,
		})

		fmt.Print("\nClaude: ")
		_, err = ce.getResponse(ctx)
		if err != nil {
			return err
		}

		fmt.Println()
	}
}

func (ce *CommitExplainer) getResponse(ctx context.Context) (string, error) {
	prompt := ce.buildPromptWithHistory()

	ce.agent.logger.Printf("Sending conversation turn to Claude (history length: %d)", len(ce.conversationHistory))

	request := claudecode.QueryRequest{
		Prompt: prompt,
		Options: &claudecode.Options{
			AllowedTools:   []string{"Read", "Grep", "LS"},
			PermissionMode: stringPtr("acceptEdits"),
			Cwd:            stringPtr(ce.agent.folder),
			OutputFormat:   outputFormatPtr(claudecode.OutputFormatStreamJSON),
			Verbose:        boolPtr(false),
			MaxTurns:       intPtr(15),
		},
	}

	messageChan, errorChan := claudecode.QueryStreamWithRequest(ctx, request)

	var responseText strings.Builder
	var lastPrintedLength int

	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				fmt.Println()
				response := strings.TrimSpace(responseText.String())

				ce.conversationHistory = append(ce.conversationHistory, ConversationMessage{
					Role:    "assistant",
					Content: response,
				})

				ce.agent.logger.Printf("Response received, length: %d characters", len(response))
				return response, nil
			}

			if message.Type() == claudecode.MessageTypeAssistant {
				for _, block := range message.Content() {
					if textBlock, ok := block.(*claudecode.TextBlock); ok {
						responseText.WriteString(textBlock.Text)

						currentText := responseText.String()
						if len(currentText) > lastPrintedLength {
							newText := currentText[lastPrintedLength:]
							fmt.Print(newText)
							lastPrintedLength = len(currentText)
						}
					}
				}
			}

		case err := <-errorChan:
			if err != nil {
				ce.agent.logger.Printf("Error getting response: %v", err)
				return "", fmt.Errorf("failed to get response: %w", err)
			}

		case <-ctx.Done():
			ce.agent.logger.Printf("Context cancelled")
			return "", ctx.Err()
		}
	}
}

func (ce *CommitExplainer) buildPromptWithHistory() string {
	var prompt strings.Builder

	prompt.WriteString(ce.agent.systemPrompt)
	prompt.WriteString("\n\n")

	prompt.WriteString("Here is the commit you need to analyze:\n\n")
	prompt.WriteString("<commit_code>\n")
	prompt.WriteString(ce.commitDiff)
	prompt.WriteString("\n</commit_code>\n\n")

	prompt.WriteString(fmt.Sprintf("The codebase can be found at: %s\n\n", ce.agent.folder))

	if len(ce.conversationHistory) > 0 {
		prompt.WriteString("Conversation history:\n\n")
		for _, msg := range ce.conversationHistory {
			if msg.Role == "user" {
				prompt.WriteString(fmt.Sprintf("<user>\n%s\n</user>\n\n", msg.Content))
			} else {
				prompt.WriteString(fmt.Sprintf("<assistant>\n%s\n</assistant>\n\n", msg.Content))
			}
		}
	}

	return prompt.String()
}
