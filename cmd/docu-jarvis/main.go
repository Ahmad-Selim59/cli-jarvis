package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/udemy/docu-jarvis-cli/internal/agent"
	"github.com/udemy/docu-jarvis-cli/internal/config"
	"github.com/udemy/docu-jarvis-cli/internal/git"
	"github.com/udemy/docu-jarvis-cli/internal/help"
	"github.com/udemy/docu-jarvis-cli/internal/settings"
	"github.com/udemy/docu-jarvis-cli/internal/system_prompts"
	"github.com/udemy/docu-jarvis-cli/internal/updater"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var updateDocsFiles string
	var writeDocsTopics string
	var debugMode bool
	var checkStagingMode bool
	var configMode bool
	var showHelp bool
	var explainCommit string
	var doUpdate bool
	var checkVersion bool
	var customPrompt string

	flag.StringVar(&updateDocsFiles, "update-docs", "", "Update existing documentation (files or 'all')")
	flag.StringVar(&writeDocsTopics, "write-docs", "", "Write new documentation for specified topics (comma-separated)")
	flag.BoolVar(&debugMode, "debug", false, "Debug mode: find which commit caused a bug")
	flag.BoolVar(&checkStagingMode, "check-staging", false, "Review staged code quality")
	flag.BoolVar(&configMode, "config", false, "Edit configuration (repo URL, code standards)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.StringVar(&explainCommit, "explain", "", "Explain a specific commit interactively")
	flag.BoolVar(&doUpdate, "update", false, "Update to the latest version")
	flag.BoolVar(&checkVersion, "version", false, "Show version and check for updates")
	flag.StringVar(&customPrompt, "custom", "", "Custom prompt for updating documentation (use with -update-docs)")
	flag.Parse()

	if showHelp {
		args := flag.Args()
		if len(args) > 0 {
			topic := strings.ToLower(args[0])
			switch topic {
			case "update-docs", "update":
				help.PrintUpdateDocsHelp()
				return nil
			case "write-docs", "write":
				help.PrintWriteDocsHelp()
				return nil
			case "debug":
				help.PrintDebugHelp()
				return nil
			case "check-staging", "check", "staging":
				help.PrintCheckStagingHelp()
				return nil
			case "explain":
				help.PrintExplainHelp()
				return nil
			default:
				fmt.Printf("Unknown help topic: %s\n\n", topic)
				help.PrintUsage()
				return nil
			}
		}
		help.PrintUsage()
		return nil
	}

	if updateDocsFiles == "-help" || updateDocsFiles == "help" {
		help.PrintUpdateDocsHelp()
		return nil
	}
	if writeDocsTopics == "-help" || writeDocsTopics == "help" {
		help.PrintWriteDocsHelp()
		return nil
	}

	if configMode {
		return runConfigMode()
	}

	if checkVersion {
		return runVersionCheck()
	}

	if doUpdate {
		return runUpdate()
	}

	if updater.ShouldCheckForUpdates() {
		go func() {
			updater.AutoCheckForUpdates(updater.GetCurrentVersion(), true)
			updater.UpdateLastCheckTime()
		}()
	}

	modesActive := 0
	if updateDocsFiles != "" {
		modesActive++
	}
	if writeDocsTopics != "" {
		modesActive++
	}
	if debugMode {
		modesActive++
	}
	if checkStagingMode {
		modesActive++
	}
	if explainCommit != "" {
		modesActive++
	}

	if modesActive == 0 {
		help.PrintUsage()
		return fmt.Errorf("please specify a command")
	}

	if modesActive > 1 {
		return fmt.Errorf("cannot use multiple modes at the same time")
	}

	if customPrompt != "" && updateDocsFiles == "" {
		return fmt.Errorf("-custom flag can only be used with -update-docs")
	}

	ctx := context.Background()

	if checkStagingMode {
		args := flag.Args()
		if len(args) > 0 && strings.ToLower(args[0]) == "settings" {
			return runCheckStagingSettings()
		}
		return runCheckStagingMode(ctx)
	}

	if explainCommit != "" {
		args := flag.Args()
		var initialQuestion string
		if len(args) > 0 {
			initialQuestion = strings.Join(args, " ")
		}
		return runExplainMode(ctx, explainCommit, initialQuestion)
	}

	fmt.Println("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Cloning repository...")
	repo := git.NewRepo(cfg.RepoURL)
	repoName := cfg.GetRepoName()

	folder, err := repo.Clone(repoName)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if debugMode {
		args := flag.Args()
		if len(args) < 3 {
			help.PrintDebugHelp()
			return fmt.Errorf("debug mode requires 3 arguments: <from-date> <to-date> <bug-description>")
		}
		fromDate := args[0]
		toDate := args[1]
		bugDescription := args[2]
		return runDebugMode(ctx, folder, repo, fromDate, toDate, bugDescription)
	}

	if updateDocsFiles != "" {
		files := parseTopics(updateDocsFiles)
		return runUpdateMode(ctx, folder, repo, files, customPrompt)
	}

	if writeDocsTopics != "" {
		topics := parseTopics(writeDocsTopics)
		return runWriteMode(ctx, folder, repo, topics)
	}

	return nil
}

func parseTopics(topicsStr string) []string {
	parts := strings.Split(topicsStr, ",")
	var topics []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			topics = append(topics, trimmed)
		}
	}
	return topics
}

func runUpdateMode(ctx context.Context, folder string, repo *git.Repo, files []string, customPrompt string) error {
	fmt.Println("\n=== UPDATE DOCUMENTATION MODE ===")

	if len(files) == 0 {
		return fmt.Errorf("no files specified - use 'all' or specify file names")
	}

	var systemPrompt string
	if customPrompt != "" {
		fmt.Println("Using custom prompt for documentation updates...")
		systemPrompt = customPrompt
	} else {
		systemPrompt = system_prompts.DocumentationUpdate
	}

	fmt.Println("Initializing agent for documentation updates...")
	ag, err := agent.New(systemPrompt, folder)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	var successCount, totalFiles int

	// Check if user wants to update all files
	if len(files) == 1 && strings.ToLower(files[0]) == "all" {
		fmt.Println("Updating ALL documentation files...")
		successCount, totalFiles, err = ag.ProcessDocuments(ctx)
		if err != nil {
			return fmt.Errorf("failed to process documents: %w", err)
		}
	} else {
		// Update specific files
		fmt.Printf("Updating %d specific files...\n", len(files))

		docsDir := filepath.Join(folder, "documentation")
		var filePaths []string
		for _, file := range files {
			if !strings.HasSuffix(file, ".md") {
				file = file + ".md"
			}
			filePaths = append(filePaths, filepath.Join(docsDir, file))
		}

		successCount, totalFiles, err = ag.UpdateSpecificDocuments(ctx, filePaths)
		if err != nil {
			return fmt.Errorf("failed to update documents: %w", err)
		}
	}

	if successCount == totalFiles && totalFiles > 0 {
		fmt.Println("\nAll documents processed successfully")

		hasChanges, err := repo.HasChanges()
		if err != nil {
			return fmt.Errorf("failed to check for changes: %w", err)
		}

		if hasChanges {
			fmt.Println("\nCreating pull request...")
			if err := repo.CreatePR(); err != nil {
				return fmt.Errorf("failed to create PR: %w", err)
			}
		} else {
			fmt.Println("\nNo changes detected in documentation")
		}
	} else {
		fmt.Printf("\nSome documents failed to process (%d/%d successful)\n", successCount, totalFiles)
	}

	fmt.Println("\n✓ Documentation update completed!")
	return nil
}

func runWriteMode(ctx context.Context, folder string, repo *git.Repo, topics []string) error {
	fmt.Printf("\n=== WRITE DOCUMENTATION MODE ===\n")
	fmt.Printf("Topics to document: %v\n", topics)

	systemPrompt := system_prompts.DocumentationWrite

	fmt.Println("\nInitializing agent...")
	ag, err := agent.New(systemPrompt, folder)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Println("Checking for existing documentation...")
	matches, err := ag.CheckExistingDocs(ctx, topics)
	if err != nil {
		return fmt.Errorf("failed to check existing docs: %w", err)
	}

	var topicsToWrite []string
	var topicsToUpdate []string
	var topicsToSkip []string

	hasConflicts := false
	for _, match := range matches {
		if match.IsMatch {
			hasConflicts = true
			fmt.Printf("\nOH NO!!!!  Topic '%s' already documented in: %s\n", match.Topic, match.ExistingFile)
		}
	}

	if hasConflicts {
		fmt.Println("\nWhat would you like to do with existing documentation?")
		fmt.Println("  1. Write new files (keep existing)")
		fmt.Println("  2. Update existing files")
		fmt.Println("  3. Skip existing topics")
		fmt.Print("\nChoice (1/2/3): ")

		var choice string
		fmt.Scanln(&choice)

		for _, match := range matches {
			if match.IsMatch {
				switch choice {
				case "1":
					topicsToWrite = append(topicsToWrite, match.Topic)
				case "2":
					topicsToUpdate = append(topicsToUpdate, match.Topic)
				case "3":
					topicsToSkip = append(topicsToSkip, match.Topic)
					fmt.Printf("  Skipping: %s\n", match.Topic)
				default:
					return fmt.Errorf("invalid choice: %s", choice)
				}
			} else {
				topicsToWrite = append(topicsToWrite, match.Topic)
			}
		}
	} else {
		topicsToWrite = topics
	}

	var writeSuccess, writeTotal int
	var updateSuccess, updateTotal int

	if len(topicsToWrite) > 0 {
		fmt.Printf("\nWriting documentation for %d new topics...\n", len(topicsToWrite))
		writeSuccess, writeTotal, err = ag.WriteDocumentation(ctx, topicsToWrite)
		if err != nil {
			return fmt.Errorf("failed to write documentation: %w", err)
		}
	}

	if len(topicsToUpdate) > 0 {
		fmt.Printf("\nUpdating documentation for %d existing topics...\n", len(topicsToUpdate))

		updatePrompt := system_prompts.DocumentationUpdate

		updateAgent, err := agent.New(updatePrompt, folder)
		if err != nil {
			return fmt.Errorf("failed to create update agent: %w", err)
		}

		var filesToUpdate []string
		for _, match := range matches {
			if match.IsMatch {
				for _, topic := range topicsToUpdate {
					if topic == match.Topic {
						filePath := filepath.Join(folder, "documentation", match.ExistingFile)
						filesToUpdate = append(filesToUpdate, filePath)
						break
					}
				}
			}
		}

		updateSuccess, updateTotal, err = updateAgent.UpdateSpecificDocuments(ctx, filesToUpdate)
		if err != nil {
			return fmt.Errorf("failed to update documentation: %w", err)
		}
	}

	successCount := writeSuccess + updateSuccess
	totalTopics := writeTotal + updateTotal + len(topicsToSkip)

	if successCount > 0 {
		if successCount == totalTopics {
			fmt.Println("\nAll topics documented successfully")
		} else {
			fmt.Printf("\nSome topics failed, but %d/%d succeeded\n", successCount, totalTopics)
		}

		hasChanges, err := repo.HasChanges()
		if err != nil {
			return fmt.Errorf("failed to check for changes: %w", err)
		}

		if hasChanges {
			fmt.Println("\nCreating pull request with new documentation...")
			if err := repo.CreatePR(); err != nil {
				return fmt.Errorf("failed to create PR: %w", err)
			}
		} else {
			fmt.Println("\nNo new documentation files were created")
		}
	} else {
		fmt.Println("\nAll topics failed - no documentation created")
	}

	fmt.Println("\n✓ Documentation writing completed!")
	return nil
}

func runDebugMode(ctx context.Context, folder string, repo *git.Repo, fromDate, toDate, bugDescription string) error {
	fmt.Println("\n=== DEBUG MODE ===")
	fmt.Printf("Date range: %s to %s\n", fromDate, toDate)
	fmt.Printf("Bug: %s\n\n", bugDescription)

	fmt.Println("Fetching commits in date range...")
	commits, err := repo.GetCommitsBetweenDates(fromDate, toDate)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No commits found in the specified date range")
		return nil
	}

	fmt.Printf("Found %d commits to analyze\n", len(commits))

	systemPrompt := system_prompts.DebugAnalysis

	fmt.Println("\nAnalyzing commits with Claude AI (concurrently)...")
	ag, err := agent.New(systemPrompt, folder)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	analysis, err := ag.AnalyzeBugInCommits(ctx, commits, bugDescription)
	if err != nil {
		return fmt.Errorf("failed to analyze commits: %w", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("DEBUG ANALYSIS RESULTS!!!")
	fmt.Println(strings.Repeat("=", 70))

	if !analysis.IsLikely {
		fmt.Println("\nOH NO!!!!  Could not definitively identify the bug-causing commit")
		fmt.Printf("\nExplanation:\n%s\n", analysis.Explanation)
	} else {
		fmt.Println("\n✓ Likely bug-causing commit identified:")
		fmt.Println()
		fmt.Printf("Commit Hash:    %s\n", analysis.CommitHash)
		fmt.Printf("Author:         %s\n", analysis.Author)
		fmt.Printf("Date:           %s\n", analysis.Date)
		fmt.Printf("Message:        %s\n", analysis.CommitMsg)
		fmt.Printf("Confidence:     %d%%\n", analysis.Confidence)
		fmt.Println()
		fmt.Println("Explanation:")
		fmt.Println(strings.Repeat("-", 70))
		fmt.Println(analysis.Explanation)
		fmt.Println(strings.Repeat("-", 70))
		fmt.Println()
		fmt.Printf("To view the commit:\n  git show %s\n", analysis.CommitHash)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("\n✓ Debug analysis completed!")
	return nil
}

func runConfigMode() error {
	s, err := settings.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	if err := s.InteractiveEdit(); err != nil {
		return fmt.Errorf("failed to edit config: %w", err)
	}

	return nil
}

func runCheckStagingSettings() error {
	fmt.Println("\n=== CODE STANDARDS SETTINGS ===")
	fmt.Println("Note: Use 'docu-jarvis -config' to edit all settings including code standards")
	fmt.Println()

	return runConfigMode()
}

func runCheckStagingMode(ctx context.Context) error {
	fmt.Println("\n=== CHECK STAGING MODE ===")

	settings, err := settings.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	if settings.IsEmpty() {
		fmt.Println("OH NO!!!!  No code standards configured!")
		fmt.Println("\nPlease configure your code standards first:")
		fmt.Println("  docu-jarvis -check-staging settings")
		fmt.Println()
		return fmt.Errorf("code standards not configured")
	}

	fmt.Printf("Loaded code standards from: %s\n", settings.GetPath())

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	repo := git.NewRepo("")
	repo.SetLocalPath(cwd)

	fmt.Println("Getting staged changes...")
	stagedDiff, err := repo.GetStagedDiff()
	if err != nil {
		return fmt.Errorf("failed to get staged changes: %w", err)
	}

	if strings.TrimSpace(stagedDiff) == "" {
		fmt.Println("No staged changes found!")
		fmt.Println("\nStage some changes first:")
		fmt.Println("  git add <files>")
		return nil
	}

	fmt.Printf("Found staged changes (%d bytes)\n", len(stagedDiff))

	systemPrompt := system_prompts.AssertCodeQuality

	fmt.Println("Reviewing code with Claude AI...")
	ag, err := agent.New(systemPrompt, cwd)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	review, err := ag.ReviewStagedCode(ctx, stagedDiff, settings.CodeStandards)
	if err != nil {
		return fmt.Errorf("failed to review code: %w", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("CODE QUALITY REVIEW")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	fmt.Println(review.FullResponse)
	fmt.Println()

	if review.ComplianceStatus != "" {
		fmt.Println(strings.Repeat("=", 70))
		fmt.Printf("COMPLIANCE STATUS: %s\n", review.ComplianceStatus)
		fmt.Println(strings.Repeat("=", 70))
	}

	if review.Recommendations != "" {
		fmt.Println("\nRECOMMENDATIONS:")
		fmt.Println(strings.Repeat("-", 70))
		fmt.Println(review.Recommendations)
		fmt.Println(strings.Repeat("-", 70))
	}

	fmt.Println("\n✓ Code review completed!")
	return nil
}

func runVersionCheck() error {
	currentVersion := updater.GetCurrentVersion()
	fmt.Printf("Docu-Jarvis version: %s\n", currentVersion)
	fmt.Println("\nChecking for updates...")

	updater.AutoCheckForUpdates(currentVersion, false)
	return nil
}

func runUpdate() error {
	currentVersion := updater.GetCurrentVersion()
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Println("Checking for updates...")

	err := updater.UpdateToLatest(currentVersion)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println("\n✓ Update completed successfully!")
	fmt.Println("Please restart docu-jarvis to use the new version")
	return nil
}

func runExplainMode(ctx context.Context, commitHash, initialQuestion string) error {
	fmt.Println("\n=== COMMIT EXPLAINER MODE ===")
	fmt.Printf("Commit: %s\n", commitHash)

	fmt.Println("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Println("Cloning repository...")
	repo := git.NewRepo(cfg.RepoURL)
	repoName := cfg.GetRepoName()

	folder, err := repo.Clone(repoName)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Println("Fetching commit details...")
	commitDiff, err := repo.GetCommitDiff(commitHash)
	if err != nil {
		return fmt.Errorf("failed to get commit diff: %w", err)
	}

	systemPrompt := system_prompts.CommitExplainer

	fmt.Println("Initializing AI agent...")
	ag, err := agent.New(systemPrompt, folder)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	explainer := agent.NewCommitExplainer(ag, commitHash, commitDiff)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("Explaining commit: %s\n", commitHash)
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	if err := explainer.StartConversation(ctx, initialQuestion); err != nil {
		return fmt.Errorf("conversation error: %w", err)
	}

	return nil
}
