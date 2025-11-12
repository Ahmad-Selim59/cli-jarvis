package settings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	configDirName       = ".docu-jarvis"
	configFileName      = "config"
	codeStandardsKey    = "code_standards"
	repoURLKey          = "repo"
	githubTokenKey      = "github_token"
)

type Settings struct {
	RepoURL       string
	CodeStandards string
	GitHubToken   string
	configPath    string
}

func Load() (*Settings, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, configDirName)
	configPath := filepath.Join(configDir, configFileName)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		template := `# Docu-Jarvis Configuration
# Lines starting with # are comments

# Repository URL (required for documentation commands)
repo = https://github.com/your-org/your-repo.git

# GitHub Personal Access Token (required for private repos and updates)
# Create at: https://github.com/settings/tokens with 'repo' scope
github_token = ghp_your_token_here

# Code Quality Standards (one per line, used by -check-staging)
# Uncomment and customize these or add your own:
# code_standards = All functions must have documentation comments
# code_standards = Use meaningful variable names
# code_standards = Handle all errors explicitly
# code_standards = No magic numbers - use named constants
`
		if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
			return nil, fmt.Errorf("failed to create config template: %w", err)
		}
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	settings := &Settings{
		configPath: configPath,
	}

	var codeStandardsLines []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case repoURLKey:
				settings.RepoURL = value
			case githubTokenKey:
				settings.GitHubToken = value
			case codeStandardsKey:
				codeStandardsLines = append(codeStandardsLines, value)
			}
		}
	}

	settings.CodeStandards = strings.Join(codeStandardsLines, "\n")

	return settings, nil
}

func (s *Settings) GetPath() string {
	return s.configPath
}

func (s *Settings) IsEmpty() bool {
	return strings.TrimSpace(s.CodeStandards) == ""
}

func (s *Settings) GetRepoURL() string {
	return s.RepoURL
}

func (s *Settings) GetGitHubToken() string {
	if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		return envToken
	}
	return s.GitHubToken
}

func (s *Settings) InteractiveEdit() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		if _, err := exec.LookPath("vim"); err == nil {
			editor = "vim"
		} else if _, err := exec.LookPath("nano"); err == nil {
			editor = "nano"
		} else {
			editor = "vi"
		}
	}

	fmt.Printf("\nOpening Docu-Jarvis config in %s...\n", editor)
	fmt.Printf("File: %s\n", s.configPath)
	fmt.Println("\nEdit the configuration, then save and exit.")
	fmt.Println("Format: key = value")
	fmt.Println()

	cmd := exec.Command(editor, s.configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	reloaded, err := Load()
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	*s = *reloaded

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("\nCurrent settings:")
	fmt.Println(strings.Repeat("-", 60))
	if s.RepoURL != "" {
		fmt.Printf("Repository: %s\n", s.RepoURL)
	} else {
		fmt.Println("Repository: (not configured)")
	}
	if s.CodeStandards != "" {
		fmt.Printf("\nCode Standards:\n%s\n", s.CodeStandards)
	} else {
		fmt.Println("\nCode Standards: (not configured)")
	}
	fmt.Println(strings.Repeat("-", 60))

	return nil
}

